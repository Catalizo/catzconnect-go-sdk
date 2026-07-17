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

	ChannelEmail Channel = "Email"

	TemplateOtp    Template = "Otp"
	TemplateCustom Template = "Custom"
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
}

// EnvValues lets callers pass credentials explicitly instead of reading
// them from the environment. Pass a nil *EnvValues to Send to fall back to
// environment variables.
type EnvValues struct {
	APIKey          string
	PrivateKey      string
	ServerPublicKey string
}
