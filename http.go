package catzconnect

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const userAgent = "catzconnect-go-sdk/2.0.0"

// httpClient posts encrypted payloads to the API.
type httpClient struct {
	client *http.Client
	// allowInsecureHTTP accepts a plain-http base URL to a non-local host
	// (WithAllowInsecureHTTP).
	allowInsecureHTTP bool
}

// localHosts never leave the machine (10.0.2.2 is the Android emulator's host).
var localHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true, "10.0.2.2": true}

// insecureHTTPFromEnv reports CATZCONNECT_ALLOW_INSECURE_HTTP set to 1/true/yes.
func insecureHTTPFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CATZCONNECT_ALLOW_INSECURE_HTTP"))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// CheckBaseURL accepts https:// always and plain http:// only for localhost,
// 127.0.0.1, ::1 and 10.0.2.2, or when allowInsecure is set, so the API key
// is never sent unencrypted by mistake. It returns a KindValidation *Error.
func CheckBaseURL(raw string, allowInsecure bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return &Error{Kind: KindValidation, Message: "CATZCONNECT_BASE_URL is not a valid URL: " + raw}
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		if allowInsecure || localHosts[strings.ToLower(u.Hostname())] {
			return nil
		}
		return &Error{Kind: KindValidation, Message: "CATZCONNECT_BASE_URL uses plain http:// to " + u.Hostname() +
			": the API key would travel unencrypted. Use https://, or set CATZCONNECT_ALLOW_INSECURE_HTTP=1 " +
			"or catzconnect.New(catzconnect.WithAllowInsecureHTTP()) to allow it anyway"}
	}
	return &Error{Kind: KindValidation, Message: "CATZCONNECT_BASE_URL must start with https:// (got " + u.Scheme + ":)"}
}

// A bounded wait. The default client this replaced had no timeout at all, so a
// stalled connection could hang the caller forever.
func newHTTPClient() *httpClient {
	return &httpClient{client: &http.Client{Timeout: 30 * time.Second}}
}

// config resolves the API key and base URL the way the Rust SDK does.
//
// Both are trimmed. An API key read from a .env file often carries a trailing
// newline or space: Go's HTTP client refuses a header containing a newline,
// and a trailing space makes the server's exact-match key lookup fail. The
// base URL loses a trailing slash, which would otherwise produce //sdk/send.
//
// A plain-http base URL is refused unless the host is local or allowInsecure
// (or CATZCONNECT_ALLOW_INSECURE_HTTP) is set; see CheckBaseURL.
func config(env *EnvValues, allowInsecure bool) (apiKey, baseURL string, err error) {
	raw := ""
	if env != nil {
		raw = env.APIKey
	} else {
		raw = os.Getenv("CATZCONNECT_API_KEY")
		if raw == "" {
			return "", "", &Error{Kind: KindMissingEnv, Message: "API_KEY"}
		}
	}

	apiKey = strings.TrimSpace(raw)
	if apiKey == "" {
		return "", "", &Error{Kind: KindMissingEnv, Message: "API_KEY is empty"}
	}

	baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("CATZCONNECT_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://api.catzconnect.com"
	}
	if err := CheckBaseURL(baseURL, allowInsecure || insecureHTTPFromEnv()); err != nil {
		return "", "", err
	}
	return apiKey, baseURL, nil
}

func (h *httpClient) post(path string, body *encryptedPayload, env *EnvValues) (map[string]any, error) {
	return h.postCtx(context.Background(), path, body, env)
}

// postCtx is post with a context, so a caller can cancel or bound the wait.
func (h *httpClient) postCtx(ctx context.Context, path string, body *encryptedPayload, env *EnvValues) (map[string]any, error) {
	return h.postWithHeaders(ctx, path, body, env, nil)
}

// postWithHeaders is postCtx with extra request headers (Idempotency-Key).
func (h *httpClient) postWithHeaders(ctx context.Context, path string, body *encryptedPayload, env *EnvValues, extra map[string]string) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	apiKey, baseURL, err := config(env, h.allowInsecureHTTP)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, &Error{Kind: KindHTTP, Message: err.Error(), Err: err}
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	res, err := h.client.Do(req)
	if err != nil {
		return nil, &Error{Kind: KindHTTP, Message: err.Error(), Err: err}
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &Error{Kind: KindHTTP, Message: err.Error(), Err: err}
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apiError(res.StatusCode, respBody)
	}

	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}
	return out, nil
}

// postPublic posts plain JSON without an API key (the push device
// endpoints, authorised by the project's publishable key).
func (h *httpClient) postPublic(ctx context.Context, baseURL, path string, body map[string]any) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("CATZCONNECT_BASE_URL"))
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.catzconnect.com"
	}
	if err := CheckBaseURL(baseURL, h.allowInsecureHTTP || insecureHTTPFromEnv()); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, &Error{Kind: KindHTTP, Message: err.Error(), Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	res, err := h.client.Do(req)
	if err != nil {
		return nil, &Error{Kind: KindHTTP, Message: err.Error(), Err: err}
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &Error{Kind: KindHTTP, Message: err.Error(), Err: err}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apiError(res.StatusCode, respBody)
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}
	if ok, isBool := out["success"].(bool); isBool && !ok {
		return nil, apiError(res.StatusCode, respBody)
	}
	return out, nil
}
