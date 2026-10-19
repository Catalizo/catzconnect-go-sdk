package catzconnect

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

type fakeAPI struct {
	t      *testing.T
	server keypair
	client keypair
	paths  []string
	bodies []map[string]any
	auth   []string
	status int
	reply  func(path string) any
}

func newFakeAPI(t *testing.T) (*fakeAPI, *EnvValues) {
	f := &fakeAPI{t: t, server: newKeypair(t), client: newKeypair(t), status: 200}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var enc encryptedPayload
		if err := json.Unmarshal(raw, &enc); err != nil {
			t.Fatal(err)
		}
		f.paths = append(f.paths, r.URL.Path)
		f.auth = append(f.auth, r.Header.Get("Authorization")+"|"+r.Header.Get("Content-Type")+"|"+r.Header.Get("User-Agent"))
		f.bodies = append(f.bodies, serverDecrypt(t, f.server, f.client, &enc))
		w.WriteHeader(f.status)
		b, _ := json.Marshal(f.reply(r.URL.Path))
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CATZCONNECT_BASE_URL", srv.URL)
	return f, &EnvValues{APIKey: "live_k", PrivateKey: b64(f.client.priv), ServerPublicKey: b64(f.server.pub)}
}

// seal encrypts like the server would for an encrypted data envelope.
func (f *fakeAPI) seal(v any) map[string]any {
	shared, _ := curve25519.X25519(f.server.priv, f.client.pub)
	master := blake2b.Sum256(shared)
	key := blake2b.Sum256(append(master[:], []byte(keyLabel)...))
	aead, _ := chacha20poly1305.New(key[:])
	nonce := make([]byte, 12)
	rand.Read(nonce)
	plain, _ := json.Marshal(v)
	return map[string]any{
		"nonce":      base64.StdEncoding.EncodeToString(nonce),
		"ciphertext": base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plain, nil)),
	}
}

var testCall = map[string]any{
	"id": "c1", "engine": "catz", "mode": "video", "status": "ringing", "room": "c1",
	"participants": []any{map[string]any{"user_id": "alice", "role": "caller", "state": "joined"}},
	"created_at":   "2026-10-18T00:00:00Z",
}

func TestCallsCreateGetTokenEnd(t *testing.T) {
	f, env := newFakeAPI(t)
	f.reply = func(path string) any {
		switch path {
		case "/sdk/calls/create":
			return map[string]any{"success": true, "call": testCall}
		case "/sdk/calls/get":
			return map[string]any{"success": true, "message": "ok", "data": map[string]any{"call": testCall}}
		case "/sdk/calls/token":
			return map[string]any{"success": true, "data": f.seal(map[string]any{"token": "ct1.a.b", "expires_at": 1760000000, "ws_url": "wss://x/rtc/v1/ws"})}
		default:
			return map[string]any{"success": true, "message": "Call ended"}
		}
	}
	calls := New().Calls().WithEnv(env)
	ctx := context.Background()

	call, err := calls.Create(ctx, CreateCallInput{Mode: CallModeVideo, From: "alice", To: []string{"bob"},
		Engine: "catz", Ring: Bool(false), Metadata: map[string]any{"order": 7}, MaxParticipants: 4})
	if err != nil {
		t.Fatal(err)
	}
	if call.ID != "c1" || call.Participants[0].UserID != "alice" {
		t.Fatalf("call = %+v", call)
	}
	b := f.bodies[0]
	if b["mode"] != "video" || b["from"] != "alice" || b["engine"] != "catz" || b["ring"] != false ||
		b["max_participants"] != float64(4) || b["ts"] == nil || fmt.Sprint(b["to"]) != "[bob]" {
		t.Fatalf("create body = %v", b)
	}
	if f.auth[0] != "Bearer live_k|application/json|catzconnect-go-sdk/1.5.0" {
		t.Fatalf("headers = %s", f.auth[0])
	}

	if got, err := calls.Get(ctx, "c1"); err != nil || got.Status != "ringing" {
		t.Fatalf("get: %v %v", got, err)
	}
	if f.bodies[1]["call_id"] != "c1" {
		t.Fatalf("get body = %v", f.bodies[1])
	}

	tok, err := calls.Token(ctx, "alice", 600)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token != "ct1.a.b" || tok.ExpiresAt != "1760000000" || tok.WSURL != "wss://x/rtc/v1/ws" {
		t.Fatalf("token = %+v", tok)
	}
	if f.bodies[2]["user_id"] != "alice" || f.bodies[2]["ttl_seconds"] != float64(600) {
		t.Fatalf("token body = %v", f.bodies[2])
	}

	end, err := calls.End(ctx, "c1", " done ")
	if err != nil || end.Message != "Call ended" || f.bodies[3]["reason"] != "done" {
		t.Fatalf("end: %+v %v %v", end, err, f.bodies[3])
	}
	want := []string{"/sdk/calls/create", "/sdk/calls/get", "/sdk/calls/token", "/sdk/calls/end"}
	if strings.Join(f.paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths = %v", f.paths)
	}
}

func TestCallsAPIErrorCode(t *testing.T) {
	f, env := newFakeAPI(t)
	f.status = 402
	f.reply = func(string) any {
		return map[string]any{"success": false, "message": "Your plan has used its call minutes.", "code": "minutes_cap_reached"}
	}
	_, err := New().Calls().WithEnv(env).Create(context.Background(), CreateCallInput{Mode: CallModeAudio, From: "a", To: []string{"b"}})
	var ce *Error
	if !errors.As(err, &ce) || !errors.Is(err, ErrAPI) || ce.Status != 402 || ce.Code != "minutes_cap_reached" {
		t.Fatalf("want typed API error with code, got %v", err)
	}

	f.status = 200
	f.reply = func(string) any { return map[string]any{"success": false, "message": "no", "code": "plan_required"} }
	_, err = New().Calls().WithEnv(env).Token(context.Background(), "a", 0)
	if !errors.As(err, &ce) || ce.Code != "plan_required" {
		t.Fatalf("success:false must be an error, got %v", err)
	}
}

func TestCallsValidation(t *testing.T) {
	ok := CreateCallInput{Mode: CallModeAudio, From: "a", To: []string{"b"}}
	many := make([]string, 50)
	for i := range many {
		many[i] = fmt.Sprintf("u%d", i)
	}
	cases := []struct {
		in   func(CreateCallInput) CreateCallInput
		want string
	}{
		{func(c CreateCallInput) CreateCallInput { c.Mode = "screen"; return c }, "mode"},
		{func(c CreateCallInput) CreateCallInput { c.From = " "; return c }, "Missing 'from'"},
		{func(c CreateCallInput) CreateCallInput { c.From = strings.Repeat("x", 129); return c }, "128"},
		{func(c CreateCallInput) CreateCallInput { c.From = "a\x01"; return c }, "control"},
		{func(c CreateCallInput) CreateCallInput { c.To = nil; return c }, "at least one"},
		{func(c CreateCallInput) CreateCallInput { c.To = many; return c }, "at most 49"},
		{func(c CreateCallInput) CreateCallInput { c.To = []string{"b", "b"}; return c }, "twice"},
		{func(c CreateCallInput) CreateCallInput { c.To = []string{"a"}; return c }, "caller"},
		{func(c CreateCallInput) CreateCallInput {
			c.Metadata = map[string]any{"x": strings.Repeat("y", 4100)}
			return c
		}, "4 KB"},
		{func(c CreateCallInput) CreateCallInput { c.To = []string{"b", "c"}; c.MaxParticipants = 2; return c }, "do not fit"},
	}
	for _, c := range cases {
		_, err := BuildCreateCallPayload(c.in(ok))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want %q, got %v", c.want, err)
		}
	}
	if _, err := BuildCreateCallPayload(func() CreateCallInput { c := ok; c.To = many[:49]; return c }()); err != nil {
		t.Errorf("49 callees: %v", err)
	}
	for _, ttl := range []int{59, 86401, -1} {
		if _, err := BuildCallTokenPayload("a", ttl); err == nil {
			t.Errorf("ttl %d accepted", ttl)
		}
	}
	if p, err := BuildCallTokenPayload("a", 0); err != nil || len(p) != 1 {
		t.Errorf("default ttl: %v %v", p, err)
	}
	// A validation failure never reaches the network.
	if _, err := New().Calls().Create(context.Background(), CreateCallInput{}); !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

// The server sends provider as an object {id, label, name} (null for catz).
func TestCallProviderObject(t *testing.T) {
	var c Call
	raw := `{"id":"c2","engine":"livekit","provider":{"id":"6f1c","label":"LK main","name":"livekit"},"mode":"video","status":"ringing","room":"r","participants":[],"created_at":"x"}`
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	if c.Provider == nil || c.Provider.ID != "6f1c" || c.Provider.Name != "livekit" || c.Provider.Label == nil || *c.Provider.Label != "LK main" {
		t.Fatalf("provider = %+v", c.Provider)
	}
	var d Call
	if err := json.Unmarshal([]byte(`{"id":"c1","engine":"catz","provider":null,"mode":"audio","status":"ringing","room":"c1","participants":[],"created_at":"x"}`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Provider != nil {
		t.Fatalf("catz provider = %+v", d.Provider)
	}
}
