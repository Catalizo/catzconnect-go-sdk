// Package catzconnect is a secure, minimal SDK for sending encrypted
// communication requests (e.g., email OTP) to the CatzConnect API.
//
// It mirrors the CatzConnect TypeScript SDK: a stateless design (no init
// required), automatic payload validation, end-to-end payload encryption
// (X25519 ECDH + BLAKE2b key derivation + ChaCha20-Poly1305 AEAD), and API-key
// auth via a Bearer token.
package catzconnect

import (
	"errors"
	"fmt"
)

// CatzConnect is the SDK client.
type CatzConnect struct {
	http *httpClient
}

// New returns a new CatzConnect client.
func New() *CatzConnect {
	return &CatzConnect{http: &httpClient{}}
}

// Default is the package-level client, mirroring the `catzconnect` singleton
// exported by the TypeScript SDK.
var Default = New()

// Send validates, encrypts, and sends a communication request using the
// package-level Default client. This mirrors calling `catzconnect.send(...)`
// in the TypeScript SDK.
func Send(input SendInput, env *EnvValues) (map[string]any, error) {
	return Default.Send(input, env)
}

// Send validates, encrypts, and sends a communication request.
//
// If env is nil, credentials and configuration are read from the environment:
// CATZCONNECT_API_KEY, CATZCONNECT_PRIVATE_KEY, CATZCONNECT_SERVER_PUBLIC_KEY,
// and optionally CATZCONNECT_BASE_URL.
func (c *CatzConnect) Send(input SendInput, env *EnvValues) (map[string]any, error) {
	if err := verifyPayload(input); err != nil {
		return nil, err
	}

	fp := finalPayload{
		MessageType: input.Type,
		Channel:     input.Channel,
		Template:    input.Template,
		Identity:    input.Identity,
		To:          input.Payload.To,
		Otp:         input.Payload.Otp,
		Subject:     input.Payload.Subject,
		Body:        input.Payload.Body,
	}

	enc, err := encrypt(fp, env)
	if err != nil {
		return nil, err
	}
	if enc == nil {
		return nil, errors.New("Encryption failed")
	}

	res, err := c.http.post("/sdk/send", enc, env)
	if err != nil {
		return nil, fmt.Errorf("Failed to send %s.%s.%s: %w", input.Type, input.Channel, input.Template, err)
	}

	return res, nil
}
