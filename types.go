package catzconnect

import (
	"encoding/json"
	"fmt"
)

// MessageType is the kind of message being sent.
type MessageType string

// Channel is the delivery channel.
type Channel string

// Template is the message template.
type Template string

// Allowed values for MessageType, Channel, and Template. These mirror the
// string literal unions in the TypeScript SDK.
const (
	MessageTypeVerification  MessageType = "Verification"
	MessageTypeTransactional MessageType = "Transactional"
	MessageTypeNotification  MessageType = "Notification"
	// MessageTypePromotional is accepted by the SMS channel only.
	MessageTypePromotional MessageType = "Promotional"

	ChannelEmail    Channel = "Email"
	ChannelWhatsApp Channel = "WhatsApp"
	ChannelPush     Channel = "Push"
	ChannelSMS      Channel = "SMS"
	// ChannelVoice places a phone call that speaks the message
	// (text-to-speech). "voice" is accepted too.
	ChannelVoice Channel = "Voice"

	TemplateOtp          Template = "Otp"
	TemplateCustom       Template = "Custom"
	TemplateNotification Template = "Notification"
)

// Any other Template value on the Email channel is the name of an email
// template created in the panel (Email → Templates), e.g.
// Template("Order shipped"); its {{variables}} are filled from Payload.Data.
// On the SMS channel it is the name of an SMS template from the panel.

// isBuiltinTemplate reports whether t is one of the reserved names.
func isBuiltinTemplate(t Template) bool {
	return t == TemplateOtp || t == TemplateCustom || t == TemplateNotification
}

// SendInput is the input to Send.
type SendInput struct {
	Channel  Channel     `json:"channel"`
	Type     MessageType `json:"type"`
	Template Template    `json:"template"`
	// Identity is the sender identity; ignored by the SMS channel.
	Identity string      `json:"identity"`
	Payload  SendPayload `json:"payload"`
	// Language picks the language of the built-in (default) Otp/Verification
	// templates: en, de, ja, ar or hi. A BCP-47 tag such as "de-DE" is
	// accepted and sent as "de". Empty: left out of the request (the
	// server's default, English), or the client's WithDefaultLanguage.
	// Panel templates are not affected; Voice data["language"] still
	// overrides the spoken language; WhatsApp needs the Authentication
	// template approved in that language.
	Language string `json:"language,omitempty"`
	// IdempotencyKey makes a send safe to retry: the same key with the same
	// request within 24 hours returns the first answer instead of sending
	// again (sent as the Idempotency-Key header and the idempotency_key
	// field). 1–255 visible ASCII characters. A different request with a
	// used key fails with code idempotency_conflict.
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	// Reference is your own id for the message (≤ 128 characters), to find
	// it again with Messages().List.
	Reference string `json:"reference,omitempty"`
}

// WhatsAppHeader is a template header: text, media (Link or a Meta media
// ID) or a location.
type WhatsAppHeader struct {
	Type      string   `json:"type"` // text | image | video | document | location
	Text      string   `json:"text,omitempty"`
	Link      string   `json:"link,omitempty"`
	ID        string   `json:"id,omitempty"`
	Filename  string   `json:"filename,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Name      string   `json:"name,omitempty"`
	Address   string   `json:"address,omitempty"`
}

// WhatsAppButton is a dynamic button parameter of an approved template.
type WhatsAppButton struct {
	Index   int    `json:"index"`
	Type    string `json:"type"` // url | quick_reply | copy_code | otp
	Text    string `json:"text,omitempty"`
	Payload string `json:"payload,omitempty"`
	Code    string `json:"code,omitempty"`
}

// WhatsAppMedia is a media message (Template "Custom"): exactly one of Link
// (https) or ID.
type WhatsAppMedia struct {
	Type     string `json:"type"` // image | document | video | audio
	Link     string `json:"link,omitempty"`
	ID       string `json:"id,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// WhatsAppOptions holds approved-template parameters (Template = the Meta
// template name) or a media message (Template "Custom").
type WhatsAppOptions struct {
	Language string          `json:"language,omitempty"`
	Header   *WhatsAppHeader `json:"header,omitempty"`
	// Body is the positional {{1}}… values; NamedBody the values of a NAMED
	// template. Set at most one.
	Body      []string          `json:"-"`
	NamedBody map[string]string `json:"-"`
	Buttons   []WhatsAppButton  `json:"buttons,omitempty"`
	Media     *WhatsAppMedia    `json:"media,omitempty"`
}

// MarshalJSON sends Body or NamedBody as "body".
func (o WhatsAppOptions) MarshalJSON() ([]byte, error) {
	type plain WhatsAppOptions
	out := struct {
		plain
		Body any `json:"body,omitempty"`
	}{plain: plain(o)}
	if len(o.NamedBody) > 0 {
		out.Body = o.NamedBody
	} else if len(o.Body) > 0 {
		out.Body = o.Body
	}
	return json.Marshal(out)
}

// TrackingOverride (Email) turns open / click tracking on or off for one
// message; nil fields inherit the domain / organization setting.
type TrackingOverride struct {
	Opens  *bool `json:"opens,omitempty"`
	Clicks *bool `json:"clicks,omitempty"`
}

// SendPayload holds the operation-specific fields. Which fields are
// required depends on the operation (see Send / verifyPayload).
type SendPayload struct {
	// To is an email address (Email), a phone number with country code
	// (WhatsApp), an FCM token (Push) or an E.164 number such as
	// +919876543210 (SMS, Voice).
	To      string `json:"to,omitempty"`
	Otp     string `json:"otp,omitempty"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`

	// Push: key/value pairs delivered to the app. Email with a panel
	// template: the values for its {{variables}}.
	Title string            `json:"title,omitempty"`
	Data  map[string]string `json:"data,omitempty"`
	Image string            `json:"image,omitempty"`
	Link  string            `json:"link,omitempty"`
	// DeviceKey is the device's X25519 public key, base64. When set, the
	// notification is sealed so only that device can read it.
	DeviceKey string `json:"device_key,omitempty"`
	// ExternalUserID (push) sends to every device your app registered for
	// this user with POST /push/register, instead of one token in To. Give
	// one of To or ExternalUserID. Each device's registered key is used for
	// encryption, so DeviceKey must be empty.
	ExternalUserID string `json:"external_user_id,omitempty"`

	// Gateway (SMS) sends through this gateway — its id or label — instead
	// of routing. It must be one of the API key's SMS gateways.
	Gateway string `json:"gateway,omitempty"`
	// Sender (SMS) overrides the sender: an E.164 number or a 2–11
	// character sender ID. Only with Gateway, when that gateway allows it.
	Sender string `json:"sender,omitempty"`
	// DLTTemplateID (SMS) is the India DLT content template id for Custom
	// bodies and the default OTP text. Required for +91 numbers unless the
	// panel template carries one.
	DLTTemplateID string `json:"dlt_template_id,omitempty"`

	// Provider (Voice) calls through this phone provider — its id or label —
	// instead of routing. It must be one of the API key's call providers.
	// Voice options travel in Data: "language" (e.g. "en-US"), "voice"
	// ("female", "male" or a provider voice name) and "repeat" ("1"–"3").
	Provider string `json:"provider,omitempty"`

	// WhatsApp: approved-template parameters or a media message.
	WhatsApp *WhatsAppOptions `json:"whatsapp,omitempty"`
	// Tracking (Email): open / click tracking for this message.
	Tracking *TrackingOverride `json:"tracking,omitempty"`
	// Topic (Email) is a subscription topic key of your organization
	// (Email → Preferences in the panel): its opt-outs apply and the
	// message's unsubscribe link unsubscribes from that topic only.
	Topic string `json:"topic,omitempty"`
	// Push (Push) sends to every device, a topic, or later. The answer's
	// data then has broadcast_id, audience, status "scheduled" and send_at;
	// follow it with Push().Broadcast / Cancel. Server keys only.
	Push *PushOptions `json:"push,omitempty"`
}

// PushOptions (Push): a broadcast, a topic send or a scheduled push.
// Broadcast and Topic leave To / ExternalUserID empty; SendAt (RFC 3339
// with a zone, up to 30 days ahead) also works with To or ExternalUserID.
type PushOptions struct {
	Broadcast bool   `json:"broadcast,omitempty"`
	Topic     string `json:"topic,omitempty"`
	SendAt    string `json:"send_at,omitempty"`
}

// EnvValues lets callers pass credentials explicitly instead of reading
// them from the environment. Pass a nil *EnvValues to Send to fall back to
// environment variables.
//
// Printing an EnvValues (fmt %v, %+v, %#v, %s, String, GoString) never shows
// the API key or the private key.
type EnvValues struct {
	APIKey          string
	PrivateKey      string
	ServerPublicKey string
}

// redactSecret keeps the first 4 characters of a long secret and its length.
func redactSecret(s string) string {
	if s == "" {
		return "[empty]"
	}
	prefix := ""
	if len(s) > 8 {
		prefix = s[:4]
	}
	return fmt.Sprintf("%s…[redacted, %d chars]", prefix, len(s))
}

// String redacts the API key and the private key.
func (e EnvValues) String() string {
	return fmt.Sprintf("{APIKey:%s PrivateKey:[redacted] ServerPublicKey:%s}", redactSecret(e.APIKey), e.ServerPublicKey)
}

// GoString redacts the API key and the private key (%#v).
func (e EnvValues) GoString() string {
	return fmt.Sprintf("catzconnect.EnvValues{APIKey:%q, PrivateKey:\"[redacted]\", ServerPublicKey:%q}",
		redactSecret(e.APIKey), e.ServerPublicKey)
}

// Format makes every fmt verb (%v, %+v, %#v, %s, %q, …) print the redacted form.
func (e EnvValues) Format(f fmt.State, verb rune) {
	if verb == 'v' && f.Flag('#') {
		_, _ = fmt.Fprint(f, e.GoString())
		return
	}
	_, _ = fmt.Fprint(f, e.String())
}
