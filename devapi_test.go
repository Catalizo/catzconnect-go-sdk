package catzconnect

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testID = "6f0c1b9e-1111-4222-8333-944455556666"

func TestSendIdempotencyAndReference(t *testing.T) {
	f, env := newFakeAPI(t)
	var idemHeader string
	f.reply = func(string) any {
		return map[string]any{"success": true, "message": "Success", "data": map[string]any{"message_id": testID, "status": "Queued"}}
	}
	// Wrap the fake server to see the header.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idemHeader = r.Header.Get("Idempotency-Key")
		raw, _ := io.ReadAll(r.Body)
		var enc encryptedPayload
		_ = json.Unmarshal(raw, &enc)
		f.bodies = append(f.bodies, serverDecrypt(t, f.server, f.client, &enc))
		b, _ := json.Marshal(f.reply(r.URL.Path))
		w.Write(b)
	}))
	defer srv.Close()
	t.Setenv("CATZCONNECT_BASE_URL", srv.URL)

	opens := false
	res, err := New().Send(SendInput{Channel: ChannelEmail, Type: MessageTypeTransactional, Template: TemplateCustom, Identity: "hello",
		Payload:        SendPayload{To: "a@b.co", Subject: "Hi", Body: "x", Tracking: &TrackingOverride{Opens: &opens}},
		IdempotencyKey: "order-42", Reference: "order 42"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := res["data"].(map[string]any); d["message_id"] != testID {
		t.Fatalf("data: %v", res)
	}
	b := f.bodies[len(f.bodies)-1]
	if idemHeader != "order-42" || b["idempotency_key"] != "order-42" || b["reference"] != "order 42" {
		t.Fatalf("header %q body %v", idemHeader, b)
	}
	if tr, _ := b["tracking"].(map[string]any); tr["opens"] != false {
		t.Fatalf("tracking: %v", b["tracking"])
	}
}

func TestSendWithoutNewOptionsIsUnchanged(t *testing.T) {
	p, err := buildPayload(SendInput{Channel: ChannelEmail, Type: MessageTypeTransactional, Template: TemplateCustom, Identity: "h", Payload: SendPayload{To: "a@b.co", Subject: "s", Body: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"idempotency_key", "reference", "tracking", "whatsapp"} {
		if _, ok := p[k]; ok {
			t.Errorf("%s should be absent", k)
		}
	}
	if _, err := buildPayload(SendInput{Channel: ChannelEmail, Type: MessageTypeTransactional, Template: TemplateCustom, Identity: "h",
		Payload: SendPayload{To: "a@b.co", Subject: "s", Body: "b"}, IdempotencyKey: "has space"}); err == nil {
		t.Error("bad idempotency key accepted")
	}
}

func TestWhatsAppTemplatesAndMedia(t *testing.T) {
	in := SendInput{Channel: ChannelWhatsApp, Type: MessageTypeTransactional, Template: "order_update", Identity: "919578456444",
		Payload: SendPayload{To: "+919876543210", WhatsApp: &WhatsAppOptions{Language: "en_US", Header: &WhatsAppHeader{Type: "text", Text: "#1"},
			Body: []string{"Ravi", "#1"}, Buttons: []WhatsAppButton{{Index: 0, Type: "url", Text: "abc"}}}}}
	if err := verifyPayload(in); err != nil {
		t.Fatal(err)
	}
	p, err := buildPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	var back map[string]any
	_ = json.Unmarshal(raw, &back)
	wa := back["whatsapp"].(map[string]any)
	if wa["language"] != "en_US" || len(wa["body"].([]any)) != 2 {
		t.Fatalf("whatsapp: %v", wa)
	}
	named := WhatsAppOptions{NamedBody: map[string]string{"first_name": "Ana"}}
	nb, _ := json.Marshal(named)
	if !strings.Contains(string(nb), `"body":{"first_name":"Ana"}`) {
		t.Fatalf("named body: %s", nb)
	}

	media := SendInput{Channel: ChannelWhatsApp, Type: MessageTypeNotification, Template: TemplateCustom, Identity: "919578456444",
		Payload: SendPayload{To: "+919876543210", Body: "Invoice", WhatsApp: &WhatsAppOptions{Media: &WhatsAppMedia{Type: "document", Link: "https://x.example/i.pdf", Filename: "i.pdf"}}}}
	if err := verifyPayload(media); err != nil {
		t.Fatal(err)
	}
	if p, _ := buildPayload(media); p["whatsapp"] == nil || p["body"] != "Invoice" {
		t.Fatalf("media payload: %v", p)
	}

	bad := []SendInput{
		{Channel: ChannelWhatsApp, Type: MessageTypeTransactional, Template: "Order Update", Identity: "9195784", Payload: SendPayload{To: "+919876543210"}},
		{Channel: ChannelWhatsApp, Type: MessageTypeTransactional, Template: TemplateCustom, Identity: "9195784", Payload: SendPayload{To: "+919876543210", WhatsApp: &WhatsAppOptions{Media: &WhatsAppMedia{Type: "audio", Link: "https://x/a", Caption: "no"}}}},
		{Channel: ChannelWhatsApp, Type: MessageTypeTransactional, Template: TemplateCustom, Identity: "9195784", Payload: SendPayload{To: "+919876543210", WhatsApp: &WhatsAppOptions{Media: &WhatsAppMedia{Type: "image", Link: "http://x/a"}}}},
		{Channel: ChannelWhatsApp, Type: MessageTypeTransactional, Template: "promo", Identity: "9195784", Payload: SendPayload{To: "+919876543210", WhatsApp: &WhatsAppOptions{Body: []string{"two\nlines"}}}},
		{Channel: ChannelWhatsApp, Type: MessageTypeTransactional, Template: "promo", Identity: "9195784", Payload: SendPayload{To: "+919876543210", WhatsApp: &WhatsAppOptions{Buttons: []WhatsAppButton{{Index: 0, Type: "url"}}}}},
		{Channel: ChannelSMS, Type: MessageTypeTransactional, Template: TemplateCustom, Payload: SendPayload{To: "+15550100", Body: "x", Tracking: &TrackingOverride{}}},
	}
	for i, b := range bad {
		if err := verifyPayload(b); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestErrorCodesAndDetails(t *testing.T) {
	e := apiError(400, []byte(`{"success":false,"message":"wrong","code":"otp_invalid","attempts_remaining":2}`))
	if e.Code != CodeOtpInvalid || e.Details["attempts_remaining"] != float64(2) {
		t.Fatalf("%+v", e)
	}
	if !IsRetryableCode(CodeSendingPaused) || IsRetryableCode(CodeKeyExpired) {
		t.Fatal("retryable")
	}
}

func TestMessagesOtpManage(t *testing.T) {
	f, env := newFakeAPI(t)
	c := New()
	ctx := context.Background()

	f.reply = func(string) any {
		return map[string]any{"success": true, "data": map[string]any{"message_id": testID, "channel": "sms", "status": "delivered",
			"events": []any{map[string]any{"type": "queued", "at": "x"}}}}
	}
	m, err := c.Messages().WithEnv(env).Get(ctx, testID)
	if err != nil || m.Status != "delivered" || len(m.Events) != 1 {
		t.Fatalf("%v %v", m, err)
	}
	if f.paths[0] != "/sdk/messages/get" || f.bodies[0]["message_id"] != testID {
		t.Fatalf("%v %v", f.paths, f.bodies)
	}
	if _, err := c.Messages().WithEnv(env).Get(ctx, "nope"); !errors.Is(err, ErrValidation) {
		t.Fatal("bad id accepted")
	}
	f.reply = func(string) any {
		return map[string]any{"success": true, "data": map[string]any{"messages": []any{map[string]any{"message_id": testID}}, "next_before": nil}}
	}
	l, err := c.Messages().WithEnv(env).List(ctx, ListMessagesInput{Recipient: "+15550100", Channel: "SMS", Limit: 5})
	if err != nil || len(l.Messages) != 1 {
		t.Fatal(l, err)
	}
	if f.bodies[len(f.bodies)-1]["channel"] != "sms" {
		t.Fatal(f.bodies)
	}
	if _, err := BuildListMessagesPayload(ListMessagesInput{}); err == nil {
		t.Fatal("empty list filter accepted")
	}

	f.reply = func(string) any {
		return map[string]any{"success": true, "data": map[string]any{"verification_id": testID, "status": "pending", "test": true, "code": "123456"}}
	}
	s, err := c.Otp().WithEnv(env).Start(ctx, OtpStartInput{Channel: "sms", To: "+15550100", CodeLength: 8})
	if err != nil || s.Code != "123456" {
		t.Fatal(s, err)
	}
	last := f.bodies[len(f.bodies)-1]
	if last["channel"] != "SMS" || last["code_length"] != float64(8) {
		t.Fatal(last)
	}
	if _, err := BuildOtpStartPayload(OtpStartInput{Channel: "push", To: "x"}); err == nil {
		t.Fatal("push otp accepted")
	}
	if p, _ := BuildOtpVerifyPayload(OtpVerifyInput{VerificationID: testID, Code: "123 456"}); p["code"] != "123456" {
		t.Fatal(p)
	}

	f.status = 400
	f.reply = func(string) any {
		return map[string]any{"success": false, "message": "wrong", "code": "otp_invalid", "attempts_remaining": 4}
	}
	_, err = c.Otp().WithEnv(env).Verify(ctx, OtpVerifyInput{VerificationID: testID, Code: "000000"})
	var ce *Error
	if !errors.As(err, &ce) || ce.Code != CodeOtpInvalid || ce.Details["attempts_remaining"] != float64(4) {
		t.Fatal(err)
	}

	f.status = 200
	f.reply = func(string) any {
		return map[string]any{"success": true, "data": map[string]any{"id": testID, "api_key": "k", "private_key": "p", "server_public_key": "s"}}
	}
	k, err := c.Manage().WithEnv(env).CreateKey(ctx, map[string]any{"name": "CI", "kind": "test", "channels": []string{"Email"}})
	if err != nil || k.APIKey != "k" {
		t.Fatal(k, err)
	}
	if f.paths[len(f.paths)-1] != "/sdk/manage/keys/create" {
		t.Fatal(f.paths)
	}
	if _, err := c.Manage().WithEnv(env).Call(ctx, "keys", "explode", nil); err == nil {
		t.Fatal("unknown op accepted")
	}
	if _, err := c.Manage().WithEnv(env).CreateKey(ctx, map[string]any{"name": "x", "kind": "app"}); err == nil {
		t.Fatal("app kind accepted")
	}
}

func TestPushDevicesArePlainJSONWithoutAPIKey(t *testing.T) {
	var gotAuth, gotPath string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"success":true,"message":"ok","data":{"device_id":"d1","created":true}}`))
	}))
	defer srv.Close()
	tok := strings.Repeat("t", 40)
	res, err := New().PushDevices(srv.URL).Register(context.Background(), RegisterDeviceInput{PublishableKey: "pk_push_abcdef", Token: tok, Platform: "web", UserHash: "h"})
	if err != nil || res["device_id"] != "d1" {
		t.Fatal(res, err)
	}
	if gotAuth != "" || gotPath != "/push/register" || got["user_hash"] != "h" || got["token"] != tok {
		t.Fatal(gotAuth, gotPath, got)
	}
	if _, err := BuildRegisterDevicePayload(RegisterDeviceInput{PublishableKey: "pk_push_abcdef", Token: tok, Platform: "tv"}); err == nil {
		t.Fatal("bad platform")
	}
}

func TestParseWebhookEvent(t *testing.T) {
	body := []byte(`{"id":"e1","type":"sms.delivered","created_at":"x","organization_id":"o","test":true,"data":{"message_id":"m"}}`)
	ts := int64(1700000000)
	mac := hmac.New(sha256.New, []byte("whsec_test"))
	mac.Write([]byte("1700000000." + string(body)))
	h := "t=1700000000,v1=" + hex.EncodeToString(mac.Sum(nil))
	ev, err := ParseWebhookEventAt(body, h, "whsec_test", time.Unix(ts, 0), DefaultWebhookTolerance)
	if err != nil || ev.Type != "sms.delivered" || !ev.Test || ev.Data["message_id"] != "m" {
		t.Fatal(ev, err)
	}
	if _, err := ParseWebhookEventAt(body, h, "other", time.Unix(ts, 0), DefaultWebhookTolerance); err == nil {
		t.Fatal("bad secret accepted")
	}
}

func TestEmailTopicAndPushOptions(t *testing.T) {
	email := SendInput{Channel: ChannelEmail, Type: MessageTypeTransactional, Template: TemplateCustom, Identity: "h",
		Payload: SendPayload{To: "a@b.co", Subject: "s", Body: "b", Topic: "newsletter"}}
	if err := verifyPayload(email); err != nil {
		t.Fatal(err)
	}
	if p, _ := buildPayload(email); p["topic"] != "newsletter" {
		t.Fatal(p)
	}
	bad := email
	bad.Payload.Topic = "News Letter"
	if err := verifyPayload(bad); err == nil || !strings.Contains(err.Error(), "topic key") {
		t.Fatal(err)
	}
	sms := SendInput{Channel: ChannelSMS, Type: MessageTypeTransactional, Template: TemplateCustom, Payload: SendPayload{To: "+15550100", Body: "x", Topic: "newsletter"}}
	if err := verifyPayload(sms); err == nil || !strings.Contains(err.Error(), "Email channel only") {
		t.Fatal(err)
	}

	push := func(pl SendPayload) SendInput {
		return SendInput{Channel: ChannelPush, Type: MessageTypeNotification, Template: TemplateNotification, Identity: "my-firebase", Payload: pl}
	}
	in := push(SendPayload{Body: "Sale", Push: &PushOptions{Topic: "deals", SendAt: PushSendAt(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))}})
	if err := verifyPayload(in); err != nil {
		t.Fatal(err)
	}
	p, err := buildPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(p["push"])
	if string(b) != `{"topic":"deals","send_at":"2026-11-02T09:00:00Z"}` || p["to"] != nil {
		t.Fatalf("%s %v", b, p)
	}
	if err := verifyPayload(push(SendPayload{Body: "x", Push: &PushOptions{Broadcast: true}})); err != nil {
		t.Fatal(err)
	}
	tok := strings.Repeat("t", 40)
	for _, c := range []struct {
		pl   SendPayload
		want string
	}{
		{SendPayload{To: tok, Body: "x", Push: &PushOptions{Broadcast: true}}, "leave 'to'"},
		{SendPayload{Body: "x", Push: &PushOptions{Broadcast: true, Topic: "a"}}, "not both"},
		{SendPayload{Body: "x", Push: &PushOptions{Topic: "a/b"}}, "push.topic"},
		{SendPayload{Body: "x", Push: &PushOptions{Broadcast: true, SendAt: "2026-11-02 09:00"}}, "RFC 3339"},
		{SendPayload{Body: "x", Push: &PushOptions{Topic: "a"}, DeviceKey: "k"}, "device_key"},
	} {
		if err := verifyPayload(push(c.pl)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v", c.pl.Push, err)
		}
	}
	// A scheduled push to one token keeps `to`.
	if p, _ := buildPayload(push(SendPayload{To: tok, Body: "x", Push: &PushOptions{SendAt: "2026-11-02T09:00:00+02:00"}})); p["to"] != tok {
		t.Fatal(p)
	}
	for _, c := range []string{CodeInvalidTopic, CodeTopicEmailOnly, CodeRecipientSuppressed, CodeNotCancelable, CodeNotResendable, CodeWhatsAppOptedOut, CodeContentGone} {
		if c == "" {
			t.Fatal("empty code")
		}
	}
}

func TestPushBroadcastsTopicsAndSuppressions(t *testing.T) {
	f, env := newFakeAPI(t)
	c := New()
	ctx := context.Background()
	f.reply = func(path string) any {
		switch path {
		case "/sdk/push/broadcast":
			return map[string]any{"success": true, "data": map[string]any{"id": testID, "status": "dispatching", "queued": 500, "audience": "all"}}
		case "/sdk/push/cancel":
			return map[string]any{"success": true, "data": map[string]any{"broadcast_id": testID, "status": "canceled"}}
		case "/sdk/push/topics":
			return map[string]any{"success": true, "data": map[string]any{"topics": []any{map[string]any{"topic": "deals", "devices": 2}}}}
		}
		return map[string]any{"success": true, "data": map[string]any{"topic": "deals", "devices": 2}}
	}
	b, err := c.Push().WithEnv(env).Broadcast(ctx, testID)
	if err != nil || b.Queued != 500 || b.Audience != "all" {
		t.Fatal(b, err)
	}
	if r, err := c.Push().WithEnv(env).Cancel(ctx, testID); err != nil || r["status"] != "canceled" {
		t.Fatal(r, err)
	}
	if _, err := c.Push().WithEnv(env).Cancel(ctx, "x"); !errors.Is(err, ErrValidation) {
		t.Fatal("bad id accepted")
	}
	tok := strings.Repeat("t", 40)
	if _, err := c.Push().WithEnv(env).Subscribe(ctx, PushTopicChange{Identity: "my-firebase", Topic: "deals", Tokens: []string{tok}, ExternalUserID: "u1"}); err != nil {
		t.Fatal(err)
	}
	last := f.bodies[len(f.bodies)-1]
	if f.paths[len(f.paths)-1] != "/sdk/push/topic/subscribe" || last["external_user_id"] != "u1" || last["topic"] != "deals" {
		t.Fatal(f.paths, last)
	}
	if _, err := c.Push().WithEnv(env).Unsubscribe(ctx, PushTopicChange{Identity: "my-firebase", Topic: "deals", ExternalUserID: "u1"}); err != nil || f.paths[len(f.paths)-1] != "/sdk/push/topic/unsubscribe" {
		t.Fatal(err, f.paths)
	}
	if _, err := BuildPushTopicChangePayload(PushTopicChange{Identity: "p", Topic: "deals"}); err == nil {
		t.Fatal("no devices accepted")
	}
	if r, err := c.Push().WithEnv(env).Topics(ctx, "my-firebase"); err != nil || r["topics"] == nil {
		t.Fatal(r, err)
	}

	// Messages of one broadcast.
	p, err := BuildListMessagesPayload(ListMessagesInput{BroadcastID: testID})
	if err != nil || p["broadcast_id"] != testID {
		t.Fatal(p, err)
	}

	// Suppressions on every channel, topics, remove by id.
	f.reply = func(string) any { return map[string]any{"success": true, "data": map[string]any{}} }
	m := c.Manage().WithEnv(env)
	if _, err := m.AddSuppressionWith(ctx, SuppressionInput{Channel: "email", Address: "a@b.co", Topic: "newsletter", Detail: "asked"}); err != nil {
		t.Fatal(err)
	}
	last = f.bodies[len(f.bodies)-1]
	if last["topic"] != "newsletter" || last["detail"] != "asked" || last["channel"] != "email" {
		t.Fatal(last)
	}
	if _, err := m.AddSuppressionWith(ctx, SuppressionInput{Channel: "sms", Address: "+15550100", Topic: "newsletter"}); err == nil {
		t.Fatal("sms topic accepted")
	}
	if _, err := m.AddSuppression(ctx, "whatsapp", "+919876543210"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RemoveSuppressionWith(ctx, SuppressionInput{ID: testID}); err != nil || f.bodies[len(f.bodies)-1]["id"] != testID {
		t.Fatal(err, f.bodies)
	}
	if _, err := m.RemoveSuppression(ctx, "push", "user-42"); err != nil || f.bodies[len(f.bodies)-1]["channel"] != "push" {
		t.Fatal(err)
	}
}

func TestPushDeviceTopicsArePlainJSON(t *testing.T) {
	var gotPath string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"success":true,"message":"ok","data":{"topics":["deals"]}}`))
	}))
	defer srv.Close()
	tok := strings.Repeat("t", 40)
	res, err := New().PushDevices(srv.URL).SubscribeTopics(context.Background(), "pk_push_abcdef", tok, []string{"deals"})
	if err != nil || gotPath != "/push/topics/subscribe" || got["token"] != tok || res["topics"] == nil {
		t.Fatal(res, err, gotPath, got)
	}
	if _, err := New().PushDevices(srv.URL).UnsubscribeTopics(context.Background(), "pk_push_abcdef", tok, []string{"deals"}); err != nil || gotPath != "/push/topics/unsubscribe" {
		t.Fatal(err, gotPath)
	}
	if _, err := New().PushDevices(srv.URL).SubscribeTopics(context.Background(), "pk_push_abcdef", tok, nil); err == nil {
		t.Fatal("no topics accepted")
	}
	// Opens: with the device token (verified + webhook) or without (legacy).
	id := "6f0c1b9e-1111-4222-8333-944455556666"
	got = nil
	if _, err := New().PushDevices(srv.URL).OpenedWithToken(context.Background(), "pk_push_abcdef", id, tok); err != nil || gotPath != "/push/opened" || got["token"] != tok || got["message_id"] != id {
		t.Fatal(err, gotPath, got)
	}
	got = nil
	if _, err := New().PushDevices(srv.URL).Opened(context.Background(), "pk_push_abcdef", id); err != nil || got["token"] != nil {
		t.Fatal("legacy open sent a token", err, got)
	}
	if _, err := New().PushDevices(srv.URL).OpenedWithToken(context.Background(), "pk_push_abcdef", id, "short"); err == nil {
		t.Fatal("short token accepted")
	}
}

func TestWebhookEventsIncludeCrossChannel(t *testing.T) {
	have := map[string]bool{}
	for _, e := range WebhookEvents {
		have[e] = true
	}
	for _, e := range []string{"email.complained", "email.unsubscribed", "push.opened", "sms.inbound", "sms.opt_out", "whatsapp.opt_out"} {
		if !have[e] {
			t.Error(e)
		}
	}
}
