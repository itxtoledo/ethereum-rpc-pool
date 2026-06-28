//go:build integration

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var anvilURL string

func TestMain(m *testing.M) {
	anvilPath, err := exec.LookPath("anvil")
	if err != nil {
		os.Exit(0)
	}

	cmd := exec.Command(anvilPath, "--port", "8545", "--silent")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		os.Exit(0)
	}
	anvilURL = "http://localhost:8545"

	time.Sleep(2 * time.Second)

	code := m.Run()

	cmd.Process.Kill()
	cmd.Wait()
	os.Exit(code)
}

func TestAnvil_BlockNumber(t *testing.T) {
	if anvilURL == "" {
		t.Skip("anvil not available")
	}

	rpcs = []string{anvilURL}
	InitializeRPCStatus(rpcs)

	w := httptest.NewRecorder()
	body := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	RPCHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	blockNum, ok := result["result"].(string)
	if !ok || blockNum == "" {
		t.Fatalf("expected block number string, got %v", result["result"])
	}
	if blockNum[:2] != "0x" {
		t.Errorf("expected hex block number, got %s", blockNum)
	}
	t.Logf("Block number from Anvil: %s", blockNum)
}

func TestAnvil_ChainId(t *testing.T) {
	if anvilURL == "" {
		t.Skip("anvil not available")
	}

	rpcs = []string{anvilURL}
	InitializeRPCStatus(rpcs)

	w := httptest.NewRecorder()
	body := `{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	RPCHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", result)
	}

	chainId, ok := result["result"].(string)
	if !ok {
		t.Fatalf("expected chainId string, got %v", result["result"])
	}
	if chainId[:2] != "0x" {
		t.Errorf("expected hex chainId, got %s", chainId)
	}
	t.Logf("Chain ID from Anvil: %s", chainId)
}

func TestAnvil_GetBlockByNumber(t *testing.T) {
	if anvilURL == "" {
		t.Skip("anvil not available")
	}

	rpcs = []string{anvilURL}
	InitializeRPCStatus(rpcs)

	w := httptest.NewRecorder()
	body := `{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0",true],"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	RPCHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	block, ok := result["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected block object, got %v", result["result"])
	}
	if block["number"] != "0x0" {
		t.Errorf("expected block 0x0, got %v", block["number"])
	}
}

func TestAnvil_StatusEndpoint(t *testing.T) {
	if anvilURL == "" {
		t.Skip("anvil not available")
	}

	rpcs = []string{anvilURL}
	InitializeRPCStatus(rpcs)

	fetchBlockNumberFromRPC(anvilURL)

	time.Sleep(500 * time.Millisecond)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	StatusHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	rpcStatus, ok := body[anvilURL].(map[string]any)
	if !ok {
		t.Fatalf("expected status for %s in response", anvilURL)
	}
	if rpcStatus["online"] != true {
		t.Errorf("expected Anvil online, got %v", rpcStatus["online"])
	}
	if rpcStatus["blockNumber"] == "" {
		t.Error("expected blockNumber from Anvil")
	}
}

func TestAnvil_CacheHit(t *testing.T) {
	if anvilURL == "" {
		t.Skip("anvil not available")
	}

	rpcs = []string{anvilURL}
	InitializeRPCStatus(rpcs)

	body := `{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	RPCHandler(w1, req1)

	if w1.Result().StatusCode != http.StatusOK {
		t.Fatal("first request failed")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	RPCHandler(w2, req2)

	if w2.Result().StatusCode != http.StatusOK {
		t.Fatal("second request failed")
	}

	var r1, r2 map[string]any
	json.NewDecoder(w1.Result().Body).Decode(&r1)
	json.NewDecoder(w2.Result().Body).Decode(&r2)

	if r1["result"] != r2["result"] {
		t.Errorf("cache returned different result: %v vs %v", r1["result"], r2["result"])
	}
}

func TestAnvil_ProxyRequest(t *testing.T) {
	if anvilURL == "" {
		t.Skip("anvil not available")
	}

	rpcs = []string{anvilURL}
	InitializeRPCStatus(rpcs)

	w := httptest.NewRecorder()
	body := `{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	RPCHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	accounts, ok := result["result"].([]any)
	if !ok {
		t.Fatalf("expected accounts array, got %v", result["result"])
	}
	t.Logf("Accounts from Anvil: %v", accounts)
}

func TestAnvil_RoundRobinMultipleRPCs(t *testing.T) {
	if anvilURL == "" {
		t.Skip("anvil not available")
	}

	var buf bytes.Buffer
	_ = buf

	rpcs = []string{anvilURL, anvilURL, anvilURL}
	InitializeRPCStatus(rpcs)

	for i := 0; i < 9; i++ {
		w := httptest.NewRecorder()
		body := `{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		RPCHandler(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("request %d failed with status %d", i, w.Result().StatusCode)
		}
	}
}
