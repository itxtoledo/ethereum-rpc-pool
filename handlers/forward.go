package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ethereum-rpc-pool/utils"
)

type outcome int

const (
	outcomeOK outcome = iota
	// outcomeRetry means the upstream failed in a way another upstream may not.
	outcomeRetry
	// outcomeNull means a valid response with a null result, retried only for
	// methods where null is usually a lagging node (eth_getBlockByNumber).
	outcomeNull
)

func (o outcome) String() string {
	switch o {
	case outcomeOK:
		return "ok"
	case outcomeRetry:
		return "retry"
	case outcomeNull:
		return "null_result"
	}
	return "unknown"
}

// retryableRPCCodes are JSON-RPC error codes that describe a provider problem
// (rate limit, internal failure), not a problem with the request itself.
var retryableRPCCodes = map[int]bool{
	-32005: true, // limit exceeded
	-32603: true, // internal error
}

var retryableRPCMessages = []string{
	"rate limit", "too many requests", "timeout", "timed out",
	"capacity", "temporarily unavailable", "overloaded", "service unavailable",
}

// attempt records one upstream call for the logs and the final error.
type attempt struct {
	Upstream   string
	Status     int
	DurationMS int64
	Bytes      int
	Outcome    outcome
	Reason     string
}

type forwardResult struct {
	// Body is the accepted upstream response, nil when every attempt failed.
	Body []byte
	// NullBody is the last well-formed response with a null result, returned
	// when retrying for a non-null result found none.
	NullBody []byte
	Attempts []attempt
	// TimedOut is true when the total request budget ended the retries.
	TimedOut bool
}

// redactURL hides everything that can carry credentials (userinfo, path,
// query) and keeps the host plus a short fingerprint so two upstreams on the
// same host stay distinguishable in the logs.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "invalid-url#" + fingerprint(raw)
	}
	return fmt.Sprintf("%s://%s/…#%s", u.Scheme, u.Host, fingerprint(raw))
}

func fingerprint(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:6]
}

func redactedList(urls []string) []string {
	out := make([]string, len(urls))
	for i, u := range urls {
		out[i] = redactURL(u)
	}
	return out
}

// bodyForLog renders a body for the debug logs, honouring LOG_BODY_MAX_BYTES.
func bodyForLog(b []byte) string {
	max := cfg.LogBodyMaxBytes
	if max > 0 && len(b) > max {
		return string(b[:max]) + fmt.Sprintf("…(truncated, %d bytes total)", len(b))
	}
	return string(b)
}

func bodiesEnabled() bool {
	return cfg.LogBodies && Logger().Enabled(context.Background(), slog.LevelDebug)
}

// classify decides whether an upstream answer can be returned to the client.
func classify(status int, body []byte, err error, retryNull bool) (outcome, string) {
	if err != nil {
		return outcomeRetry, "transport error: " + err.Error()
	}
	if status < 200 || status > 299 {
		return outcomeRetry, fmt.Sprintf("upstream http status %d", status)
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal(body, &resp); jsonErr != nil {
		return outcomeRetry, "upstream returned invalid JSON: " + jsonErr.Error()
	}

	if resp.Error != nil {
		if retryableRPCCodes[resp.Error.Code] {
			return outcomeRetry, fmt.Sprintf("upstream json-rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		lower := strings.ToLower(resp.Error.Message)
		for _, needle := range retryableRPCMessages {
			if strings.Contains(lower, needle) {
				return outcomeRetry, fmt.Sprintf("upstream json-rpc error %d: %s", resp.Error.Code, resp.Error.Message)
			}
		}
		// A real answer about the request itself (revert, nonce too low...):
		// another upstream would say the same, so hand it to the client.
		return outcomeOK, ""
	}

	if retryNull && (len(resp.Result) == 0 || string(resp.Result) == "null") {
		return outcomeNull, "null result"
	}
	return outcomeOK, ""
}

// upstreamOrder returns the upstreams in the order they should be tried:
// round-robin rotation first, with upstreams the health check marked offline
// moved to the end (they are still tried when nothing else is left).
func upstreamOrder() []string {
	if len(rpcs) == 0 {
		return nil
	}
	first := utils.GetNextRPC(rpcs)
	start := 0
	for i, r := range rpcs {
		if r == first {
			start = i
			break
		}
	}

	statuses := GetRPCStatus()
	var online, offline []string
	for i := 0; i < len(rpcs); i++ {
		r := rpcs[(start+i)%len(rpcs)]
		if s, ok := statuses[r]; ok && !s.Timestamp.IsZero() && !s.Online {
			offline = append(offline, r)
		} else {
			online = append(online, r)
		}
	}
	return append(online, offline...)
}

// doUpstream performs one HTTP call and returns the status and body.
func doUpstream(ctx context.Context, rpcURL string, requestBody []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(requestBody))
	if err != nil {
		return 0, nil, fmt.Errorf("error creating proxy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("error reading RPC response: %w", err)
	}
	return resp.StatusCode, body, nil
}

// forward sends reqBody to the upstreams, retrying on provider failures until
// MaxAttempts or the request budget runs out.
func forward(ctx context.Context, requestID, method string, reqBody []byte, retryNull bool) forwardResult {
	log := Logger().With("request_id", requestID, "method", method)

	ctx, cancel := context.WithTimeout(ctx, cfg.TotalTimeout)
	defer cancel()

	order := upstreamOrder()
	result := forwardResult{}
	if len(order) == 0 {
		log.Error("no upstream configured")
		return result
	}

	if log.Enabled(ctx, slog.LevelDebug) {
		fields := []any{
			"max_attempts", cfg.MaxAttempts,
			"upstream_order", redactedList(order),
			"request_bytes", len(reqBody),
		}
		if bodiesEnabled() {
			fields = append(fields, "request_body", bodyForLog(reqBody))
		}
		log.Debug("forwarding request", fields...)
	}

	for i := 0; i < cfg.MaxAttempts; i++ {
		if ctx.Err() != nil {
			result.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
			break
		}

		// Back off only when an upstream is about to be tried a second time.
		if i >= len(order) && cfg.RetryBackoff > 0 {
			select {
			case <-time.After(cfg.RetryBackoff * time.Duration(i/len(order))):
			case <-ctx.Done():
			}
		}

		upstream := order[i%len(order)]
		start := time.Now()
		status, body, err := doUpstream(ctx, upstream, reqBody)
		elapsed := time.Since(start)
		UpstreamDuration.WithLabelValues(upstream).Observe(elapsed.Seconds())

		oc, reason := classify(status, body, err, retryNull)
		a := attempt{
			Upstream:   redactURL(upstream),
			Status:     status,
			DurationMS: elapsed.Milliseconds(),
			Bytes:      len(body),
			Outcome:    oc,
			Reason:     reason,
		}
		result.Attempts = append(result.Attempts, a)

		fields := []any{
			"attempt", i + 1,
			"max_attempts", cfg.MaxAttempts,
			"upstream", a.Upstream,
			"upstream_status", status,
			"duration_ms", a.DurationMS,
			"response_bytes", a.Bytes,
			"outcome", oc.String(),
		}
		if reason != "" {
			fields = append(fields, "reason", reason)
		}
		if bodiesEnabled() {
			fields = append(fields, "response_body", bodyForLog(body))
		}

		switch oc {
		case outcomeOK:
			log.Debug("upstream attempt", fields...)
			result.Body = body
			if i > 0 {
				log.Info("request recovered after retry", "attempts", i+1, "upstream", a.Upstream)
			}
			return result
		case outcomeNull:
			log.Debug("upstream attempt", fields...)
			result.NullBody = body
		default:
			log.Warn("upstream attempt failed", fields...)
		}
	}

	if ctx.Err() != nil {
		result.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	}

	if result.NullBody != nil {
		log.Debug("every attempt returned a null result, returning it", "attempts", len(result.Attempts))
		return result
	}

	summary := make([]any, 0, len(result.Attempts))
	for i, a := range result.Attempts {
		summary = append(summary, slog.Group(fmt.Sprintf("attempt_%d", i+1),
			"upstream", a.Upstream,
			"upstream_status", a.Status,
			"duration_ms", a.DurationMS,
			"reason", a.Reason,
		))
	}
	log.Error("all upstream attempts failed", append([]any{
		"attempts", len(result.Attempts),
		"timed_out", result.TimedOut,
		"request_bytes", len(reqBody),
	}, summary...)...)
	return result
}

// respond writes the outcome of forward to the client. Successful answers
// keep status 200 (JSON-RPC errors from the node included); exhausted retries
// become 502, or 504 when the request budget ran out, instead of the previous
// 400 that clients read as "your request is invalid".
func respond(w http.ResponseWriter, requestID string, id any, res forwardResult) bool {
	switch {
	case res.Body != nil:
		writeJSONResponse(w, http.StatusOK, res.Body)
		debugResponse(requestID, http.StatusOK, res.Body)
		return true
	case res.NullBody != nil:
		writeJSONResponse(w, http.StatusOK, res.NullBody)
		debugResponse(requestID, http.StatusOK, res.NullBody)
		return true
	}

	status := http.StatusBadGateway
	message := "All RPC providers failed"
	if res.TimedOut {
		status = http.StatusGatewayTimeout
		message = "RPC request timed out"
	}
	data := map[string]any{
		"request_id": requestID,
		"attempts":   len(res.Attempts),
	}
	SendErrorWithData(w, status, -32603, message, id, data)
	debugResponse(requestID, status, nil)
	return false
}

func debugResponse(requestID string, status int, body []byte) {
	log := Logger()
	if !log.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	fields := []any{"request_id", requestID, "status", status, "response_bytes", len(body)}
	if bodiesEnabled() && body != nil {
		fields = append(fields, "response_body", bodyForLog(body))
	}
	log.Debug("response sent to client", fields...)
}
