package catzconnect

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

type keypair struct{ priv, pub []byte }

func newKeypair(t *testing.T) keypair {
	t.Helper()
	priv := make([]byte, 32)
	if _, err := rand.Read(priv); err != nil {
		t.Fatal(err)
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	return keypair{priv, pub}
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// serverDecrypt is written independently from the SDK's encrypt, from the
// server's side of the exchange: server private key + client public key, as
// the server's decrypt_payload does.
func serverDecrypt(t *testing.T, server, client keypair, enc *encryptedPayload) map[string]any {
	t.Helper()
	shared, err := curve25519.X25519(server.priv, client.pub)
	if err != nil {
		t.Fatal(err)
	}
	master := blake2b.Sum256(shared)
	key := blake2b.Sum256(append(master[:], []byte("CONNECT-@-2026-HS-@-CATZ")...))
	aead, _ := chacha20poly1305.New(key[:])
	nonce, _ := base64.StdEncoding.DecodeString(enc.Nonce)
	ct, _ := base64.StdEncoding.DecodeString(enc.Ciphertext)
	plain, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		t.Fatalf("server could not decrypt: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(plain, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestServerCanDecryptWhatGoSends(t *testing.T) {
	client, server := newKeypair(t), newKeypair(t)
	env := &EnvValues{APIKey: "k", PrivateKey: b64(client.priv), ServerPublicKey: b64(server.pub)}

	p, err := buildPayload(SendInput{
		Type: MessageTypeVerification, Channel: ChannelEmail, Template: TemplateOtp,
		Identity: "user@domain.com", Payload: SendPayload{To: "a@b.com", Otp: "123456"},
	})
	if err != nil {
		t.Fatal(err)
	}
	enc, err := encrypt(p, env)
	if err != nil {
		t.Fatal(err)
	}
	got := serverDecrypt(t, server, client, enc)
	if got["otp"] != "123456" || got["channel"] != "Email" || got["ts"] == nil {
		t.Fatalf("unexpected payload: %v", got)
	}
}

// The field-dropping bug: Push fields must reach the server.
func TestPushFieldsSurviveEncryption(t *testing.T) {
	client, server := newKeypair(t), newKeypair(t)
	env := &EnvValues{APIKey: "k", PrivateKey: b64(client.priv), ServerPublicKey: b64(server.pub)}

	p, err := buildPayload(SendInput{
		Type: MessageTypeNotification, Channel: ChannelPush, Template: TemplateNotification,
		Identity: "proj", Payload: SendPayload{
			To: strings.Repeat("t", 40), Title: "Shipped", Body: "Friday",
			Data: map[string]string{"order": "42"}, Link: "https://x.com/o/42", DeviceKey: "DEVKEY",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := encrypt(p, env)
	got := serverDecrypt(t, server, client, enc)

	for _, k := range []string{"title", "body", "link", "device_key", "data"} {
		if got[k] == nil {
			t.Errorf("field %q was dropped", k)
		}
	}
	if d, _ := got["data"].(map[string]any); d["order"] != "42" {
		t.Errorf("data not preserved: %v", got["data"])
	}
}

// The likely cause of "Go does not work": keys damaged by .env files.
func TestKeysSurviveCommonDamage(t *testing.T) {
	kp := newKeypair(t)
	std := b64(kp.pub)
	urlsafe := base64.URLEncoding.EncodeToString(kp.pub)
	for name, v := range map[string]string{
		"trailing newline":   std + "\n",
		"surrounding spaces": "  " + std + "  ",
		"url-safe":           urlsafe,
		"unpadded":           strings.TrimRight(std, "="),
	} {
		got, err := key32(v)
		if err != nil || string(got) != string(kp.pub) {
			t.Errorf("%s: decoded wrong (err=%v)", name, err)
		}
	}
	if _, err := key32(b64(kp.pub[:31])); !errors.Is(err, ErrInvalidKeyLength) {
		t.Errorf("31-byte key: want ErrInvalidKeyLength, got %v", err)
	}
}

func TestRequestIsCleanDespiteMessyConfig(t *testing.T) {
	var gotAuth, gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotUA = r.Header.Get("Authorization"), r.URL.Path, r.Header.Get("User-Agent")
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	t.Setenv("CATZCONNECT_BASE_URL", srv.URL+"/ ") // trailing slash and space
	env := &EnvValues{APIKey: "live_key\n "}       // trailing newline and space

	if _, err := newHTTPClient().post("/sdk/send", &encryptedPayload{"n", "c"}, env); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer live_key" {
		t.Errorf("Authorization = %q, want the trimmed key", gotAuth)
	}
	if gotPath != "/sdk/send" {
		t.Errorf("path = %q, want /sdk/send (no double slash)", gotPath)
	}
	if !strings.HasPrefix(gotUA, "catzconnect-go-sdk/") {
		t.Errorf("User-Agent = %q", gotUA)
	}
}

func TestAPIErrorsAreTyped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"success":false}`))
	}))
	defer srv.Close()
	t.Setenv("CATZCONNECT_BASE_URL", srv.URL)

	_, err := newHTTPClient().post("/sdk/send", &encryptedPayload{"n", "c"}, &EnvValues{APIKey: "k"})
	var ce *Error
	if !errors.Is(err, ErrAPI) || !errors.As(err, &ce) || ce.Status != 401 {
		t.Fatalf("want typed 401 API error, got %v", err)
	}
}

func TestMissingEnvIsNamed(t *testing.T) {
	t.Setenv("CATZCONNECT_PRIVATE_KEY", "")
	_, err := encrypt(map[string]any{}, nil)
	if !errors.Is(err, ErrMissingEnv) || !strings.Contains(err.Error(), "PRIVATE_KEY") {
		t.Fatalf("want MissingEnv naming PRIVATE_KEY, got %v", err)
	}
}

func TestValidationErrorsAreTyped(t *testing.T) {
	_, err := Send(SendInput{Type: MessageTypeNotification, Channel: ChannelPush,
		Template: TemplateNotification, Identity: "p", Payload: SendPayload{To: strings.Repeat("t", 40)}}, nil)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("missing body: want ErrValidation, got %v", err)
	}
}
