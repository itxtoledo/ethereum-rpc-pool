package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ethereum-rpc-pool/middleware"
)

const okBody = `{"jsonrpc":"2.0","id":1,"result":"0x1"}`

// setup points the handlers at the given upstreams with a fast config and a
// logger that writes to the returned buffer. State is restored on cleanup.
func setup(t *testing.T, level slog.Level, mutate func(*Config), urls ...string) *bytes.Buffer {
	t.Helper()
	oldRPCs, oldCfg, oldLogger, oldTimeout := rpcs, cfg, logger, httpClient.Timeout

	buf := &bytes.Buffer{}
	logger = slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level}))

	c := DefaultConfig()
	c.RetryBackoff = time.Millisecond
	c.UpstreamTimeout = 2 * time.Second
	c.TotalTimeout = 5 * time.Second
	if mutate != nil {
		mutate(&c)
	}
	SetConfig(c)

	rpcs = urls
	InitializeRPCStatus(urls)

	t.Cleanup(func() {
		rpcs, cfg, logger = oldRPCs, oldCfg, oldLogger
		httpClient.Timeout = oldTimeout
	})
	return buf
}

func call(t *testing.T, method string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":[]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "req-test"))
	w := httptest.NewRecorder()
	RPCHandler(w, req)
	return w
}

func jsonServer(status int, body string, hits *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func TestRetry_RecoversFromHTTPError(t *testing.T) {
	bad := jsonServer(500, "boom", nil)
	good := jsonServer(200, okBody, nil)
	defer bad.Close()
	defer good.Close()
	buf := setup(t, slog.LevelInfo, nil, bad.URL, good.URL)

	for i := 0; i < 4; i++ { // covers both round-robin start positions
		w := call(t, "eth_sendRawTransaction")
		if w.Code != 200 || strings.TrimSpace(w.Body.String()) != okBody {
			t.Fatalf("call %d: want 200 %s, got %d %s", i, okBody, w.Code, w.Body.String())
		}
	}
	if !strings.Contains(buf.String(), "request recovered after retry") {
		t.Error("expected a 'recovered after retry' log")
	}
}

func TestRetry_RecoversFromDroppedConnection(t *testing.T) {
	dropper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	good := jsonServer(200, okBody, nil)
	defer dropper.Close()
	defer good.Close()
	setup(t, slog.LevelInfo, nil, dropper.URL, good.URL)

	for i := 0; i < 4; i++ {
		if w := call(t, "eth_estimateGas"); w.Code != 200 {
			t.Fatalf("call %d: want 200, got %d %s", i, w.Code, w.Body.String())
		}
	}
}

func TestRetry_RetriesInvalidJSONAndRateLimit(t *testing.T) {
	html := jsonServer(200, "<html>bad gateway</html>", nil)
	limited := jsonServer(200, `{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"limit exceeded"}}`, nil)
	good := jsonServer(200, okBody, nil)
	defer html.Close()
	defer limited.Close()
	defer good.Close()
	setup(t, slog.LevelInfo, func(c *Config) { c.MaxAttempts = 3 }, html.URL, limited.URL, good.URL)

	for i := 0; i < 6; i++ {
		if w := call(t, "eth_call"); w.Code != 200 || !strings.Contains(w.Body.String(), `"result":"0x1"`) {
			t.Fatalf("call %d: got %d %s", i, w.Code, w.Body.String())
		}
	}
}

func TestRetry_DoesNotRetryRealJSONRPCErrors(t *testing.T) {
	var hits atomic.Int32
	revert := `{"jsonrpc":"2.0","id":1,"error":{"code":3,"message":"execution reverted"}}`
	a := jsonServer(200, revert, &hits)
	b := jsonServer(200, okBody, &hits)
	defer a.Close()
	defer b.Close()
	setup(t, slog.LevelInfo, nil, a.URL, b.URL)

	for i := 0; i < 2; i++ {
		hits.Store(0)
		w := call(t, "eth_estimateGas")
		if w.Code != 200 {
			t.Fatalf("want 200, got %d", w.Code)
		}
		if hits.Load() != 1 {
			t.Fatalf("a JSON-RPC error answer must not be retried, upstream hits = %d", hits.Load())
		}
	}
}

func TestRetry_ExhaustedReturns502WithRequestID(t *testing.T) {
	a := jsonServer(503, "down", nil)
	b := jsonServer(502, "down", nil)
	defer a.Close()
	defer b.Close()
	buf := setup(t, slog.LevelInfo, func(c *Config) { c.MaxAttempts = 3 }, a.URL, b.URL)

	w := call(t, "eth_sendRawTransaction")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", w.Code)
	}
	var resp struct {
		Error struct {
			Code    int            `json:"code"`
			Message string         `json:"message"`
			Data    map[string]any `json:"data"`
		} `json:"error"`
		ID any `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error.Code != -32603 || resp.ID != float64(1) {
		t.Errorf("unexpected error body: %s", w.Body.String())
	}
	if resp.Error.Data["attempts"] != float64(3) {
		t.Errorf("want 3 attempts, got %v", resp.Error.Data["attempts"])
	}
	if !strings.Contains(buf.String(), "all upstream attempts failed") {
		t.Error("expected an error log summarising the attempts")
	}
}

func TestRetry_TotalTimeoutReturns504(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer slow.Close()
	setup(t, slog.LevelInfo, func(c *Config) {
		c.UpstreamTimeout = 40 * time.Millisecond
		c.TotalTimeout = 90 * time.Millisecond
		c.MaxAttempts = 10
	}, slow.URL)

	w := call(t, "eth_blockNumberSlow")
	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("want 504, got %d %s", w.Code, w.Body.String())
	}
}

func TestRetry_NullResultReturnedWhenAllNull(t *testing.T) {
	null := `{"jsonrpc":"2.0","id":1,"result":null}`
	a := jsonServer(200, null, nil)
	b := jsonServer(200, null, nil)
	defer a.Close()
	defer b.Close()
	setup(t, slog.LevelInfo, nil, a.URL, b.URL)
	// no cache for this block, so the handler proxies with null retries
	w := call(t, "eth_getBlockByNumber")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"result":null`) {
		t.Fatalf("want the null answer with 200, got %d %s", w.Code, w.Body.String())
	}
}

func TestMethodCache_DoesNotCacheFailures(t *testing.T) {
	bad := jsonServer(500, "down", nil)
	defer bad.Close()
	setup(t, slog.LevelInfo, nil, bad.URL)
	methodCacheMutex.Lock()
	delete(methodCacheStore, "eth_chainId")
	methodCacheMutex.Unlock()

	if w := call(t, "eth_chainId"); w.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", w.Code)
	}
	if _, ok := GetMethodCache("eth_chainId"); ok {
		t.Error("a failed upstream answer must not be cached")
	}
}

func TestUpstreamOrder_OfflineLast(t *testing.T) {
	setup(t, slog.LevelInfo, nil, "http://a.test", "http://b.test", "http://c.test")
	SetRPCStatus("http://a.test", "", 0, false)
	SetRPCStatus("http://b.test", "0x1", 0, true)
	SetRPCStatus("http://c.test", "0x1", 0, true)

	for i := 0; i < 6; i++ {
		order := upstreamOrder()
		if len(order) != 3 || order[2] != "http://a.test" {
			t.Fatalf("offline upstream must be last, got %v", order)
		}
	}
}

func TestDebugLogging_BodiesOnlyWhenEnabled(t *testing.T) {
	srv := jsonServer(200, okBody, nil)
	defer srv.Close()

	buf := setup(t, slog.LevelDebug, func(c *Config) { c.LogBodies = true }, srv.URL)
	call(t, "eth_chainIdDebug")
	out := buf.String()
	for _, want := range []string{"forwarding request", "upstream attempt", "response sent to client", "request_body", "response_body"} {
		if !strings.Contains(out, want) {
			t.Errorf("debug log missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, redactURL(srv.URL)) || strings.Contains(out, srv.URL+`"`) {
		t.Error("upstreams must be logged in redacted form only")
	}

	buf = setup(t, slog.LevelDebug, nil, srv.URL) // debug, bodies off
	call(t, "eth_chainIdDebug")
	if strings.Contains(buf.String(), "request_body") || strings.Contains(buf.String(), "response_body") {
		t.Error("bodies must not be logged unless LOG_BODIES is on")
	}

	buf = setup(t, slog.LevelInfo, func(c *Config) { c.LogBodies = true }, srv.URL) // bodies on, level info
	call(t, "eth_chainIdDebug")
	if strings.Contains(buf.String(), "request_body") {
		t.Error("bodies must not be logged below debug level")
	}
}

func TestBodyForLog_Truncates(t *testing.T) {
	setup(t, slog.LevelDebug, func(c *Config) { c.LogBodyMaxBytes = 5 })
	if got := bodyForLog([]byte("0123456789")); !strings.HasPrefix(got, "01234…(truncated, 10 bytes total)") {
		t.Errorf("unexpected truncation: %q", got)
	}
	cfg.LogBodyMaxBytes = 0
	if got := bodyForLog([]byte("0123456789")); got != "0123456789" {
		t.Errorf("0 must mean unlimited, got %q", got)
	}
}

func TestRedactURL_HidesCredentials(t *testing.T) {
	for _, raw := range []string{
		"https://user:pass@node.example.com/v3/SECRETKEY?apikey=SECRET2",
		"https://node.example.com/v3/SECRETKEY",
	} {
		got := redactURL(raw)
		for _, secret := range []string{"SECRETKEY", "SECRET2", "pass"} {
			if strings.Contains(got, secret) {
				t.Errorf("redactURL(%q) = %q leaks %q", raw, got, secret)
			}
		}
		if !strings.Contains(got, "node.example.com") {
			t.Errorf("redactURL(%q) = %q lost the host", raw, got)
		}
	}
	if redactURL("https://a.com/k1") == redactURL("https://a.com/k2") {
		t.Error("two upstreams on one host must stay distinguishable")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("RPC_MAX_ATTEMPTS", "5")
	t.Setenv("RPC_RETRY_BACKOFF_MS", "250")
	t.Setenv("UPSTREAM_TIMEOUT_SECONDS", "7")
	t.Setenv("REQUEST_TIMEOUT_SECONDS", "20")
	t.Setenv("LOG_BODIES", "true")
	t.Setenv("LOG_BODY_MAX_BYTES", "0")
	c, warnings := LoadConfigFromEnv()
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if c.MaxAttempts != 5 || c.RetryBackoff != 250*time.Millisecond || c.UpstreamTimeout != 7*time.Second ||
		c.TotalTimeout != 20*time.Second || !c.LogBodies || c.LogBodyMaxBytes != 0 {
		t.Errorf("unexpected config: %+v", c)
	}

	t.Setenv("RPC_MAX_ATTEMPTS", "0")
	t.Setenv("LOG_BODIES", "maybe")
	c, warnings = LoadConfigFromEnv()
	if len(warnings) != 2 || c.MaxAttempts != DefaultConfig().MaxAttempts || c.LogBodies {
		t.Errorf("invalid values must keep defaults and warn: %+v %v", c, warnings)
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{"": slog.LevelInfo, "debug": slog.LevelDebug, "DEBUG": slog.LevelDebug, "warn": slog.LevelWarn, "error": slog.LevelError}
	for in, want := range cases {
		if got, ok := ParseLogLevel(in); !ok || got != want {
			t.Errorf("ParseLogLevel(%q) = %v,%v", in, got, ok)
		}
	}
	if _, ok := ParseLogLevel("loud"); ok {
		t.Error("unknown level must report ok=false")
	}
}
