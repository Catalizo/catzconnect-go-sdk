package catzconnect

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/curve25519"
)

// Cross-language interop with the Rust SDK. Runs only with CATZ_INTEROP_DIR
// set: opens the fixture the Rust SDK encrypted, and writes one of Go's for
// the Rust test to open.
func TestGoAndRustInteroperate(t *testing.T) {
	dir := os.Getenv("CATZ_INTEROP_DIR")
	if dir == "" {
		t.Skip("CATZ_INTEROP_DIR not set")
	}

	// Rust → Go
	raw, err := os.ReadFile(filepath.Join(dir, "rust_enc.json"))
	if err != nil {
		t.Fatalf("run the Rust interop test first: %v", err)
	}
	var f map[string]string
	json.Unmarshal(raw, &f)
	dec := func(s string) []byte { b, _ := base64.StdEncoding.DecodeString(s); return b }

	got := serverDecrypt(t,
		keypair{priv: dec(f["server_priv"])},
		keypair{pub: dec(f["client_pub"])},
		&encryptedPayload{f["nonce"], f["ciphertext"]},
	)
	if got["body"] != "from rust" || got["device_key"] != "DK" {
		t.Fatalf("Rust payload decrypted wrongly: %v", got)
	}
	t.Logf("RUST -> GO: decrypted %v", got)

	// Go → Rust: same fixed keys as the Rust test ([7;32] client, [9;32] server).
	client := make([]byte, 32)
	server := make([]byte, 32)
	for i := range client {
		client[i], server[i] = 7, 9
	}
	clientPub, _ := curve25519.X25519(client, curve25519.Basepoint)
	serverPub, _ := curve25519.X25519(server, curve25519.Basepoint)

	p, err := buildPayload(SendInput{
		Type: MessageTypeNotification, Channel: ChannelPush, Template: TemplateNotification,
		Identity: "proj", Payload: SendPayload{
			To: "tok", Body: "from go", DeviceKey: "DK", Data: map[string]string{"order": "42"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	enc, err := encrypt(p, &EnvValues{APIKey: "k", PrivateKey: b64(client), ServerPublicKey: b64(serverPub)})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(map[string]string{
		"server_priv": b64(server), "client_pub": b64(clientPub),
		"nonce": enc.Nonce, "ciphertext": enc.Ciphertext,
	})
	os.WriteFile(filepath.Join(dir, "go_enc.json"), out, 0o600)
}
