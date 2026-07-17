package catzconnect

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"time"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// keyLabel is mixed into the key-derivation step and must match the value the
// server expects.
const keyLabel = "CONNECT-@-2026-HS-@-CATZ"

// finalPayload is the object that gets encrypted. It mirrors the merged object
// the TypeScript SDK builds (message metadata + payload fields); the timestamp
// (ts, milliseconds since epoch) is added here at encryption time.
type finalPayload struct {
	MessageType MessageType `json:"message_type"`
	Channel     Channel     `json:"channel"`
	Template    Template    `json:"template"`
	Identity    string      `json:"identity"`
	To          string      `json:"to,omitempty"`
	Otp         string      `json:"otp,omitempty"`
	Subject     string      `json:"subject,omitempty"`
	Body        string      `json:"body,omitempty"`
	Ts          int64       `json:"ts"`
}

// encryptedPayload is the request body sent to the API.
type encryptedPayload struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// encrypt performs X25519 (ECDH) to derive a shared secret, expands it with
// BLAKE2b into a symmetric key, and seals the JSON-encoded payload with
// ChaCha20-Poly1305 (IETF, 12-byte nonce).
func encrypt(payload finalPayload, env *EnvValues) (*encryptedPayload, error) {
	if env == nil &&
		(os.Getenv("CATZCONNECT_PRIVATE_KEY") == "" || os.Getenv("CATZCONNECT_SERVER_PUBLIC_KEY") == "") {
		return nil, errors.New("Missing keys, Make sure keys exists at Environment")
	}

	payload.Ts = time.Now().UnixMilli()

	var pk, spk string
	if env != nil {
		pk = env.PrivateKey
		spk = env.ServerPublicKey
	} else {
		pk = os.Getenv("CATZCONNECT_PRIVATE_KEY")
		spk = os.Getenv("CATZCONNECT_SERVER_PUBLIC_KEY")
	}

	clientPriv, err := base64.StdEncoding.DecodeString(pk) // 32 bytes
	if err != nil {
		return nil, err
	}
	serverPub, err := base64.StdEncoding.DecodeString(spk) // 32 bytes
	if err != nil {
		return nil, err
	}

	// ECDH: shared = scalarmult(client_priv, server_pub)
	shared, err := curve25519.X25519(clientPriv, serverPub)
	if err != nil {
		return nil, err
	}

	// master = BLAKE2b(shared, 32)
	master := blake2b.Sum256(shared)

	// key = BLAKE2b(master || label, 32)
	km := append(append([]byte{}, master[:]...), []byte(keyLabel)...)
	keyEnc := blake2b.Sum256(km)

	// AEAD encrypt (ChaCha20-Poly1305 IETF, 12-byte nonce)
	aead, err := chacha20poly1305.New(keyEnc[:])
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, chacha20poly1305.NonceSize) // 12 bytes
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	message, err := marshalNoHTMLEscape(payload)
	if err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(nil, nonce, message, nil)

	return &encryptedPayload{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

// marshalNoHTMLEscape marshals v to JSON without escaping <, >, and & so the
// output matches JavaScript's JSON.stringify. The trailing newline that
// json.Encoder appends is trimmed.
func marshalNoHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
