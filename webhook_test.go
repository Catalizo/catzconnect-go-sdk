package catzconnect

import (
	"testing"
	"time"
)

// The same vector the server's own test uses (services/webhook).
const webhookVector = "t=1700000000,v1=38877139021993b830af32feea6e18a8da83eb2f6e49ee50bd9e4cf4ca4d3789"

func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"a":1}`)
	at := time.Unix(1700000100, 0)

	if !VerifyWebhookSignatureAt(body, webhookVector, "whsec_test", at, DefaultWebhookTolerance) {
		t.Fatal("valid signature rejected")
	}
	if VerifyWebhookSignatureAt([]byte(`{"a":2}`), webhookVector, "whsec_test", at, DefaultWebhookTolerance) {
		t.Fatal("tampered body accepted")
	}
	if VerifyWebhookSignatureAt(body, webhookVector, "whsec_other", at, DefaultWebhookTolerance) {
		t.Fatal("wrong secret accepted")
	}
	if VerifyWebhookSignatureAt(body, webhookVector, "whsec_test", time.Unix(1700001000, 0), DefaultWebhookTolerance) {
		t.Fatal("stale timestamp accepted")
	}
	if VerifyWebhookSignatureAt(body, "garbage", "whsec_test", at, DefaultWebhookTolerance) {
		t.Fatal("malformed header accepted")
	}
}

func TestPushExternalUserID(t *testing.T) {
	in := SendInput{
		Channel: ChannelPush, Type: MessageTypeNotification, Template: TemplateNotification,
		Identity: "my-firebase-project",
		Payload:  SendPayload{ExternalUserID: "user-42", Body: "Hi"},
	}
	if err := verifyPayload(in); err != nil {
		t.Fatalf("external_user_id refused: %v", err)
	}
	p, err := buildPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	if p["external_user_id"] != "user-42" {
		t.Fatalf("external_user_id not sent: %v", p)
	}
	if _, ok := p["to"]; ok {
		t.Fatal("empty 'to' should be omitted")
	}

	in.Payload.To = "fcm-token-fcm-token-fcm-token-fcm-token"
	if verifyPayload(in) == nil {
		t.Fatal("both to and external_user_id accepted")
	}
	in.Payload.To = ""
	in.Payload.DeviceKey = "abc"
	if verifyPayload(in) == nil {
		t.Fatal("device_key with external_user_id accepted")
	}
}

func TestEmailTemplateByName(t *testing.T) {
	in := SendInput{
		Channel: ChannelEmail, Type: MessageTypeTransactional, Template: Template("Order shipped"),
		Identity: "noreply@example.com",
		Payload:  SendPayload{To: "user@example.com", Data: map[string]string{"name": "Ann"}},
	}
	if err := verifyPayload(in); err != nil {
		t.Fatalf("panel template refused: %v", err)
	}
	p, err := buildPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	if p["template"] != "Order shipped" {
		t.Fatalf("template not sent: %v", p)
	}
	if d, _ := p["data"].(map[string]string); d["name"] != "Ann" {
		t.Fatalf("data not sent: %v", p)
	}
}
