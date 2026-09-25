package catzconnect

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const userAgent = "catzconnect-go-sdk/1.1.0"

// httpClient posts encrypted payloads to the API.
type httpClient struct {
	client *http.Client
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
func config(env *EnvValues) (apiKey, baseURL string, err error) {
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
	return apiKey, baseURL, nil
}

func (h *httpClient) post(path string, body *encryptedPayload, env *EnvValues) (map[string]any, error) {
	apiKey, baseURL, err := config(env)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, &Error{Kind: KindHTTP, Message: err.Error(), Err: err}
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
		return nil, &Error{Kind: KindAPI, Status: res.StatusCode, Body: string(respBody)}
	}

	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}
	return out, nil
}
