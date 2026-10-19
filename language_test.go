package catzconnect

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeLanguage(t *testing.T) {
	for in, want := range map[string]string{"en": "en", "DE": "de", "de-DE": "de", "ja_JP": "ja", " ar-EG ": "ar", "hi-Latn-IN": "hi"} {
		got, err := NormalizeLanguage(in)
		if err != nil || got != want {
			t.Errorf("NormalizeLanguage(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"fr", "fr-FR", "", "-", "de--DE", "english", "d e"} {
		_, err := NormalizeLanguage(bad)
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "en, de, ja, ar, hi") {
			t.Errorf("NormalizeLanguage(%q): want a validation error naming the languages, got %v", bad, err)
		}
	}
}

func TestLanguageInPayload(t *testing.T) {
	in := SendInput{Channel: ChannelEmail, Type: MessageTypeVerification, Template: TemplateOtp, Identity: "a@b.co",
		Payload: SendPayload{To: "u@x.co", Otp: "123456"}}
	p, err := buildPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p["language"]; ok {
		t.Fatalf("language must be omitted when unset: %v", p)
	}
	in.Language = "de-DE"
	if p, _ = buildPayload(in); p["language"] != "de" {
		t.Fatalf("want language de, got %v", p)
	}
	v := voiceInput(TemplateOtp, SendPayload{To: "+14155550100", Otp: "1234", Data: map[string]string{"language": "en-US"}})
	v.Channel = ChannelVoice
	v.Language = "ja"
	p, err = buildPayload(v)
	if err != nil || p["language"] != "ja" || p["data"].(map[string]string)["language"] != "en-US" {
		t.Fatalf("voice: %v %v", p, err)
	}
	s := smsInput(MessageTypeVerification, TemplateOtp, SendPayload{To: "+14155550100", Otp: "1234"})
	s.Language = "HI"
	if p, err = buildPayload(s); err != nil || p["language"] != "hi" {
		t.Fatalf("sms: %v %v", p, err)
	}
}

func TestSendLanguageAndDefault(t *testing.T) {
	f, env := newFakeAPI(t)
	f.reply = func(string) any { return map[string]any{"success": true, "message": "ok"} }
	in := SendInput{Channel: ChannelEmail, Type: MessageTypeVerification, Template: TemplateOtp, Identity: "a@b.co",
		Payload: SendPayload{To: "u@x.co", Otp: "123456"}}

	if _, err := New().Send(in, env); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.bodies[0]["language"]; ok {
		t.Fatalf("language must be omitted: %v", f.bodies[0])
	}

	c := New(WithDefaultLanguage("ar-EG"))
	if _, err := c.Send(in, env); err != nil {
		t.Fatal(err)
	}
	if f.bodies[1]["language"] != "ar" {
		t.Fatalf("default language: %v", f.bodies[1])
	}
	in.Language = "ja"
	if _, err := c.Send(in, env); err != nil {
		t.Fatal(err)
	}
	if f.bodies[2]["language"] != "ja" {
		t.Fatalf("explicit language must win over the default: %v", f.bodies[2])
	}

	in.Language = "fr-FR"
	if _, err := c.Send(in, env); !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), `"fr-FR"`) {
		t.Fatalf("want a validation error, got %v", err)
	}
	if _, err := New(WithDefaultLanguage("xx")).Send(SendInput{Channel: in.Channel, Type: in.Type, Template: in.Template,
		Identity: in.Identity, Payload: in.Payload}, env); !errors.Is(err, ErrValidation) {
		t.Fatalf("bad default: want a validation error, got %v", err)
	}
	if len(f.bodies) != 3 {
		t.Fatalf("invalid languages must not reach the server: %d requests", len(f.bodies))
	}
}
