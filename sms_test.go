package catzconnect

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAnalyzeSMSMatchesServer(t *testing.T) {
	cases := []struct {
		text     string
		encoding string
		units    int
		segments int
	}{
		{"Your verification code is 123456", "gsm7", 32, 1},
		{strings.Repeat("a", 160), "gsm7", 160, 1},
		{strings.Repeat("a", 161), "gsm7", 161, 2},
		{strings.Repeat("a", 306), "gsm7", 306, 2},
		{strings.Repeat("a", 307), "gsm7", 307, 3},
		{strings.Repeat("€", 80), "gsm7", 160, 1},
		{strings.Repeat("€", 81), "gsm7", 162, 2},
		{strings.Repeat("a", 152) + "€" + strings.Repeat("a", 152), "gsm7", 306, 3},
		{strings.Repeat("a", 1530), "gsm7", 1530, 10},
		{strings.Repeat("a", 1531), "gsm7", 1531, 11},
		{strings.Repeat("Ж", 70), "ucs2", 70, 1},
		{strings.Repeat("Ж", 670), "ucs2", 670, 10},
		{strings.Repeat("😀", 36), "ucs2", 72, 2},
		{"", "gsm7", 0, 1},
	}
	for _, c := range cases {
		a := AnalyzeSMS(c.text)
		if a.Encoding != c.encoding || a.Units != c.units || a.Segments != c.segments {
			t.Errorf("%.20q: got %+v, want %s/%d/%d", c.text, a, c.encoding, c.units, c.segments)
		}
	}
}

func TestNormalizeE164(t *testing.T) {
	for in, want := range map[string]string{
		"+919876543210":     "+919876543210",
		" +91 98765-43210 ": "+919876543210",
		"+1 (415) 555.0100": "+14155550100",
		"919876543210":      "",
		"+0123456789":       "",
		"+1234567":          "",
		"+1234567890123456": "",
	} {
		got, ok := NormalizeE164(in)
		if got != want || ok != (want != "") {
			t.Errorf("NormalizeE164(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func smsInput(typ MessageType, tpl Template, p SendPayload) SendInput {
	return SendInput{Channel: ChannelSMS, Type: typ, Template: tpl, Payload: p}
}

func TestVerifySMS(t *testing.T) {
	ok := []SendInput{
		smsInput(MessageTypeVerification, TemplateOtp, SendPayload{To: "+14155550100", Otp: "123456"}),
		smsInput(MessageTypeVerification, "otp", SendPayload{To: "+1 415 555 0100", Otp: "AB12"}),
		smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+14155550100", Body: "Hi"}),
		smsInput(MessageTypePromotional, TemplateCustom, SendPayload{To: "+919876543210", Body: "Sale", DLTTemplateID: "1207161234567890123"}),
		smsInput(MessageTypeTransactional, "Order shipped", SendPayload{To: "+14155550100", Data: map[string]string{"id": "1"}}),
		smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+14155550100", Body: "Hi", Gateway: "Twilio US", Sender: "CATZ"}),
	}
	for _, in := range ok {
		if err := verifyPayload(in); err != nil {
			t.Errorf("%+v: unexpected error %v", in, err)
		}
	}

	bad := map[string]SendInput{
		"not an E.164":         smsInput(MessageTypeVerification, TemplateOtp, SendPayload{To: "9876543210", Otp: "1"}),
		"Missing 'to'":         smsInput(MessageTypeVerification, TemplateOtp, SendPayload{Otp: "1"}),
		"goes with type Verif": smsInput(MessageTypeTransactional, TemplateOtp, SendPayload{To: "+14155550100", Otp: "1"}),
		"up to 15 letters":     smsInput(MessageTypeVerification, TemplateOtp, SendPayload{To: "+14155550100", Otp: "12-34"}),
		"use Transactional":    smsInput(MessageTypeVerification, TemplateCustom, SendPayload{To: "+14155550100", Body: "x"}),
		"Missing 'body'":       smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+14155550100", Body: "  "}),
		"segments long":        smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+14155550100", Body: strings.Repeat("a", 1531)}),
		"control characters":   smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+14155550100", Body: "a\x07b"}),
		"India (+91)":          smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+919876543210", Body: "x"}),
		"together with":        smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+14155550100", Body: "x", Sender: "CATZ"}),
		"sender ID":            smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+14155550100", Body: "x", Gateway: "g", Sender: "C"}),
		"numeric DLT":          smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{To: "+14155550100", Body: "x", DLTTemplateID: "12a456"}),
		"SMS supports type":    smsInput(MessageTypeNotification, TemplateCustom, SendPayload{To: "+14155550100", Body: "x"}),
	}
	for want, in := range bad {
		err := verifyPayload(in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v", want, err)
		}
	}
}

func TestBuildSMSPayload(t *testing.T) {
	p, err := buildPayload(smsInput(MessageTypeTransactional, TemplateCustom, SendPayload{
		To: "+919876543210", Body: "Hello", Gateway: "g1", Sender: "CATZIN",
		DLTTemplateID: "1207161234567890123", Data: map[string]string{"a": "b"},
		Title: "ignored", Subject: "ignored",
	}))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	want := map[string]any{
		"message_type": "Transactional", "channel": "SMS", "template": "Custom", "identity": "",
		"to": "+919876543210", "body": "Hello", "gateway": "g1", "sender": "CATZIN",
		"dlt_template_id": "1207161234567890123", "data": map[string]any{"a": "b"},
	}
	if len(got) != len(want) {
		t.Fatalf("keys: got %v", got)
	}
	for k, v := range want {
		if k == "data" {
			if got[k].(map[string]any)["a"] != "b" {
				t.Errorf("data: %v", got[k])
			}
			continue
		}
		if got[k] != v {
			t.Errorf("%s: got %v want %v", k, got[k], v)
		}
	}
}

func TestParseSMSResult(t *testing.T) {
	var res map[string]any
	_ = json.Unmarshal([]byte(`{"success":true,"message":"Success","data":{"message_id":"6f1c0b9e-0000-4000-8000-000000000001","segments":2,"encoding":"ucs2"}}`), &res)
	r, err := ParseSMSResult(res)
	if err != nil || r.MessageID != "6f1c0b9e-0000-4000-8000-000000000001" || r.Segments != 2 || r.Encoding != "ucs2" || r.Message != "Success" {
		t.Fatalf("got %+v, %v", r, err)
	}
	if _, err := ParseSMSResult(map[string]any{"success": true}); err == nil {
		t.Fatal("expected an error without data")
	}
}
