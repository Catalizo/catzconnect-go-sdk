package catzconnect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// httpClient posts encrypted payloads to the CatzConnect API.
type httpClient struct{}

// post sends body to baseURL+path with a Bearer token and returns the decoded
// JSON response. The base URL comes from CATZCONNECT_BASE_URL (defaulting to
// https://api.catzconnect.com). The API key comes from env (when provided) or
// the CATZCONNECT_API_KEY environment variable.
func (h *httpClient) post(path string, body *encryptedPayload, env *EnvValues) (map[string]any, error) {
	baseURL := os.Getenv("CATZCONNECT_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.catzconnect.com"
	}

	var apiKey string
	if env != nil {
		apiKey = env.APIKey
	} else {
		apiKey = os.Getenv("CATZCONNECT_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("Missing API key in environment")
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(respBody))
	}

	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return out, nil
}
