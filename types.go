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

	ChannelEmail    Channel = "Email"
	ChannelWhatsApp Channel = "WhatsApp"
	ChannelPush     Channel = "Push"

	TemplateOtp          Template = "Otp"
	TemplateCustom       Template = "Custom"
	TemplateNotification Template = "Notification"
)

// SendInput is the input to Send.
type SendInput struct {
	Channel  Channel     `json:"channel"`
	Type     MessageType `json:"type"`
	Template Template    `json:"template"`
	Identity string      `json:"identity"`
	Payload  SendPayload `json:"payload"`
}

// SendPayload holds the operation-specific fields. Which fields are
// required depends on the operation (see Send / verifyPayload).
type SendPayload struct {
	To      string `json:"to,omitempty"`
	Otp     string `json:"otp,omitempty"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`

	// Push only.
	Title string            `json:"title,omitempty"`
	Data  map[string]string `json:"data,omitempty"`
	Image string            `json:"image,omitempty"`
	Link  string            `json:"link,omitempty"`
	// DeviceKey is the device's X25519 public key, base64. When set, the
	// notification is sealed so only that device can read it.
	DeviceKey string `json:"device_key,omitempty"`
}

// EnvValues lets callers pass credentials explicitly instead of reading
// them from the environment. Pass a nil *EnvValues to Send to fall back to
// environment variables.
type EnvValues struct {
	APIKey          string
	PrivateKey      string
	ServerPublicKey string
}
