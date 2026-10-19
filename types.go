package catzconnect

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
}

// EnvValues lets callers pass credentials explicitly instead of reading
// them from the environment. Pass a nil *EnvValues to Send to fall back to
// environment variables.
type EnvValues struct {
	APIKey          string
	PrivateKey      string
	ServerPublicKey string
}
