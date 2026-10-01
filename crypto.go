package catzconnect

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// keyLabel is mixed into key derivation and must match the server.
const keyLabel = "CONNECT-@-2026-HS-@-CATZ"

// encryptedPayload is the request body sent to the API.
type encryptedPayload struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// b64Decode is deliberately lenient, exactly like the Rust SDK.
//
// Keys reach this from .env files, secret managers and copy-paste, and each of
// those adds its own damage: a trailing newline, URL-safe characters, dropped
// padding. The strict decoder this replaced rejected all three — which is why
// a key that worked in Rust could fail in Go.
func b64Decode(s string) ([]byte, error) {
	normalised := strings.NewReplacer("-", "+", "_", "/").Replace(strings.TrimSpace(s))

	if b, err := base64.StdEncoding.DecodeString(normalised); err == nil {
		return b, nil
	}

	b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(normalised, "="))
	if err != nil {
		return nil, &Error{Kind: KindBase64, Message: err.Error(), Err: err}
	}
	return b, nil
}

// key32 decodes a key and insists on exactly 32 bytes, so a truncated or
// wrong key fails with a message that says so rather than a curve25519 error.
func key32(b64 string) ([]byte, error) {
	b, err := b64Decode(b64)
	if err != nil {
		return nil, err
	}
	if len(b) != 32 {
		return nil, &Error{
			Kind:    KindInvalidKeyLength,
			Message: fmt.Sprintf("expected 32 bytes, got %d", len(b)),
		}
	}
	return b, nil
}

// keysFrom reads each key separately, like Rust, so a missing one is named.
func keysFrom(env *EnvValues) (priv, pub string, err error) {
	if env != nil {
		return env.PrivateKey, env.ServerPublicKey, nil
	}
	priv = os.Getenv("CATZCONNECT_PRIVATE_KEY")
	if priv == "" {
		return "", "", &Error{Kind: KindMissingEnv, Message: "PRIVATE_KEY"}
	}
	pub = os.Getenv("CATZCONNECT_SERVER_PUBLIC_KEY")
	if pub == "" {
		return "", "", &Error{Kind: KindMissingEnv, Message: "SERVER_PUBLIC_KEY"}
	}
	return priv, pub, nil
}

// deriveKey: X25519 → BLAKE2b-256 → BLAKE2b-256(master ‖ label). Identical to
// the Rust SDK and to the server's decrypt_payload.
func deriveKey(priv, pub []byte) ([]byte, error) {
	shared, err := curve25519.X25519(priv, pub)
	if err != nil {
		return nil, &Error{Kind: KindEncryption, Message: err.Error(), Err: err}
	}
	master := blake2b.Sum256(shared)
	km := append(append([]byte{}, master[:]...), []byte(keyLabel)...)
	key := blake2b.Sum256(km)
	return key[:], nil
}

// encrypt seals a payload for the server. The payload is a map, as Rust's is a
// serde_json Value: every field the caller built goes through. The fixed
// struct this replaced silently dropped any field it did not list.
func encrypt(payload map[string]any, env *EnvValues) (*encryptedPayload, error) {
	privB64, pubB64, err := keysFrom(env)
	if err != nil {
		return nil, err
	}

	priv, err := key32(privB64)
	if err != nil {
		return nil, err
	}
	pub, err := key32(pubB64)
	if err != nil {
		return nil, err
	}

	key, err := deriveKey(priv, pub)
	if err != nil {
		return nil, err
	}

	fp := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		fp[k] = v
	}
	fp["ts"] = time.Now().UnixMilli()

	message, err := marshalNoHTMLEscape(fp)
	if err != nil {
		return nil, &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, &Error{Kind: KindEncryption, Err: err}
	}

	nonce := make([]byte, chacha20poly1305.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, &Error{Kind: KindEncryption, Err: err}
	}

	return &encryptedPayload{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, message, nil)),
	}, nil
}

// marshalNoHTMLEscape leaves <, > and & unescaped, matching JSON.stringify and
// serde_json, and drops the newline json.Encoder appends.
func marshalNoHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
