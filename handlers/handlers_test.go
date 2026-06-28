package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSetAndGetRPCStatus(t *testing.T) {
	rpcs = []string{"http://rpc1.example.com", "http://rpc2.example.com"}
	InitializeRPCStatus(rpcs)

	SetRPCStatus("http://rpc1.example.com", "0x100", 50, true)
	SetRPCStatus("http://rpc2.example.com", "", 0, false)

	statuses := GetRPCStatus()

	s1, ok := statuses["http://rpc1.example.com"]
	if !ok {
		t.Fatal("expected rpc1 in status")
	}
	if !s1.Online {
		t.Error("expected rpc1 online")
	}
	if s1.BlockNumber != "0x100" {
		t.Errorf("expected blockNumber 0x100, got %s", s1.BlockNumber)
	}
	if s1.ResponseTime != 50 {
		t.Errorf("expected responseTime 50, got %d", s1.ResponseTime)
	}

	s2, ok := statuses["http://rpc2.example.com"]
	if !ok {
		t.Fatal("expected rpc2 in status")
	}
	if s2.Online {
		t.Error("expected rpc2 offline")
	}
}

func TestBlockCache(t *testing.T) {
	blockData := map[string]any{
		"number": "0x200",
		"hash":   "0xabc",
	}

	SetBlockCache(blockData)
	cache := GetBlockCache()

	if cache.BlockData == nil {
		t.Fatal("expected block data in cache")
	}
	if cache.BlockData["number"] != "0x200" {
		t.Errorf("expected number 0x200, got %v", cache.BlockData["number"])
	}
	if time.Since(cache.Timestamp) > time.Second {
		t.Error("expected recent timestamp")
	}
}

func TestMethodCache(t *testing.T) {
	val := []byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`)

	SetMethodCache("eth_chainId", val, 0)
	cached, ok := GetMethodCache("eth_chainId")
	if !ok {
		t.Fatal("expected cache hit for eth_chainId")
	}
	if string(cached) != string(val) {
		t.Errorf("expected %s, got %s", val, cached)
	}

	SetMethodCache("eth_gasPrice", val, 1*time.Millisecond)
	time.Sleep(2 * time.Millisecond)
	_, ok = GetMethodCache("eth_gasPrice")
	if ok {
		t.Error("expected cache miss for expired eth_gasPrice")
	}

	_, ok = GetMethodCache("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
}

func TestSendError(t *testing.T) {
	w := httptest.NewRecorder()
	SendError(w, -32601, "Method not found", 1)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["jsonrpc"] != "2.0" {
		t.Error("expected jsonrpc 2.0")
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error object")
	}
	if errObj["code"] != float64(-32601) {
		t.Errorf("expected code -32601, got %v", errObj["code"])
	}
}

func TestRPCHandler_MethodNotAllowed(t *testing.T) {
	rpcs = []string{"http://localhost:8545"}
	InitializeRPCStatus(rpcs)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	RPCHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for PUT, got %d", resp.StatusCode)
	}
}

func TestRPCHandler_GET(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	RPCHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", resp.StatusCode)
	}
}

func TestIsNullResult(t *testing.T) {
	if !isNullResult([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`)) {
		t.Error("expected true for null result")
	}
	if isNullResult([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`)) {
		t.Error("expected false for non-null result")
	}
	if isNullResult([]byte(`invalid json`)) {
		t.Error("expected false for invalid JSON")
	}
}

func TestStatusHandler(t *testing.T) {
	rpcs = []string{"http://rpc1.example.com", "http://rpc2.example.com"}
	InitializeRPCStatus(rpcs)
	SetRPCStatus("http://rpc1.example.com", "0x10", 100, true)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	StatusHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	s1, ok := body["http://rpc1.example.com"].(map[string]any)
	if !ok {
		t.Fatal("expected rpc1 in status response")
	}
	if s1["online"] != true {
		t.Error("expected rpc1 online")
	}
}

func TestSetRPCs_TrimsWhitespace(t *testing.T) {
	SetRPCs(" http://rpc1.example.com , http://rpc2.example.com ")
	if len(rpcs) != 2 {
		t.Fatalf("expected 2 RPCs, got %d: %v", len(rpcs), rpcs)
	}
	if rpcs[0] != "http://rpc1.example.com" {
		t.Errorf("expected trimmed rpc1, got %q", rpcs[0])
	}
	if rpcs[1] != "http://rpc2.example.com" {
		t.Errorf("expected trimmed rpc2, got %q", rpcs[1])
	}
}

func TestHandleMethodCache_Integration(t *testing.T) {
	rpcs = []string{}
	InitializeRPCStatus(rpcs)

	SetMethodCache("net_version", []byte(`{"jsonrpc":"2.0","id":1,"result":"1"}`), 0)

	w := httptest.NewRecorder()
	reqBody := []byte(`{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}`)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	RPCHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
