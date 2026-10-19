package catzconnect

import (
	"strings"
	"testing"
)

func voiceInput(tpl Template, p SendPayload) SendInput {
	return SendInput{Channel: "voice", Type: MessageTypeVerification, Template: tpl, Payload: p}
}

func TestVoicePayload(t *testing.T) {
	in := voiceInput("otp", SendPayload{To: "+1 (415) 555-0100", Otp: "123456",
		Data: map[string]string{"language": "en-US", "voice": "female", "repeat": "02"}, Provider: "Twilio main"})
	in.Channel = canonicalChannel(in.Channel)
	if err := verifyPayload(in); err != nil {
		t.Fatal(err)
	}
	p, err := buildPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"message_type": "Verification", "channel": "Voice", "template": "Otp", "identity": "",
		"to": "+14155550100", "otp": "123456", "provider": "Twilio main"}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("%s = %v, want %v", k, p[k], v)
		}
	}
	if d := p["data"].(map[string]string); d["repeat"] != "2" || d["language"] != "en-US" {
		t.Errorf("data = %v", d)
	}
}

func TestVoiceValidation(t *testing.T) {
	custom := func(p SendPayload) SendInput {
		return SendInput{Channel: ChannelVoice, Type: MessageTypeNotification, Template: TemplateCustom, Payload: p}
	}
	bad := []struct {
		in   SendInput
		want string
	}{
		{custom(SendPayload{To: "4155550100", Body: "hi"}), "E.164"},
		{custom(SendPayload{To: "+14155550100"}), "Missing 'body'"},
		{custom(SendPayload{To: "+14155550100", Body: strings.Repeat("a", 1001)}), "1000"},
		{custom(SendPayload{To: "+14155550100", Body: "a\x07"}), "control"},
		{custom(SendPayload{To: "+14155550100", Body: "hi", Data: map[string]string{"repeat": "4"}}), "repeat"},
		{custom(SendPayload{To: "+14155550100", Body: "hi", Data: map[string]string{"repeat": "0"}}), "repeat"},
		{custom(SendPayload{To: "+14155550100", Body: "hi", Data: map[string]string{"language": "english"}}), "language"},
		{voiceInput(TemplateOtp, SendPayload{To: "+14155550100"}), "Missing 'otp'"},
		{voiceInput(TemplateOtp, SendPayload{To: "+14155550100", Otp: "12-34"}), "15 letters"},
	}
	for _, c := range bad {
		c.in.Channel = canonicalChannel(c.in.Channel)
		err := verifyPayload(c.in)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want %q, got %v", c.want, err)
		}
	}
	if err := verifyPayload(custom(SendPayload{To: "+919876543210", Body: strings.Repeat("é", 1000)})); err != nil {
		t.Errorf("1000 characters must pass: %v", err)
	}
	if err := verifyPayload(custom(SendPayload{To: "+14155550100", Body: "hi"})); err != nil {
		t.Error(err)
	}
	// Panel template: data only.
	if err := verifyPayload(SendInput{Channel: ChannelVoice, Type: MessageTypeTransactional, Template: "Appointment",
		Payload: SendPayload{To: "+14155550100", Data: map[string]string{"time": "10:00"}}}); err != nil {
		t.Error(err)
	}
}

func TestVoiceResult(t *testing.T) {
	r, err := ParseVoiceResult(map[string]any{"success": true, "message": "Queued", "data": map[string]any{"message_id": "v1"}})
	if err != nil || r.MessageID != "v1" || r.Message != "Queued" {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := ParseVoiceResult(map[string]any{"success": true}); err == nil {
		t.Fatal("want error without message_id")
	}
	if _, err := New().SendVoice(SendInput{Channel: ChannelSMS}, nil); err == nil {
		t.Fatal("SendVoice must refuse other channels")
	}
}
