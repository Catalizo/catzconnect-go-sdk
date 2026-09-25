// Package catzconnect is a secure, minimal SDK for sending encrypted
// communication requests — email, WhatsApp and push — to the CatzConnect API.
//
// It is structured like the Rust SDK: validate, build the payload for the
// channel, encrypt (X25519 + BLAKE2b + ChaCha20-Poly1305), and send with a
// Bearer API key. Stateless; nothing needs initialising.
package catzconnect

// CatzConnect is the SDK client.
type CatzConnect struct {
	http *httpClient
}

// New returns a new client.
func New() *CatzConnect {
	return &CatzConnect{http: newHTTPClient()}
}

// Default is the package-level client used by Send.
var Default = New()

// Send validates, encrypts and sends using the package-level client.
func Send(input SendInput, env *EnvValues) (map[string]any, error) {
	return Default.Send(input, env)
}

// Send validates, encrypts and sends a request.
//
// With a nil env, credentials come from CATZCONNECT_API_KEY,
// CATZCONNECT_PRIVATE_KEY and CATZCONNECT_SERVER_PUBLIC_KEY, and the endpoint
// from CATZCONNECT_BASE_URL (default https://api.catzconnect.com).
//
// Every error is a *Error; see errors.go.
func (c *CatzConnect) Send(input SendInput, env *EnvValues) (map[string]any, error) {
	if err := verifyPayload(input); err != nil {
		return nil, &Error{Kind: KindValidation, Message: err.Error(), Err: err}
	}

	payload, err := buildPayload(input)
	if err != nil {
		return nil, err
	}

	enc, err := encrypt(payload, env)
	if err != nil {
		return nil, err
	}

	return c.http.post("/sdk/send", enc, env)
}

// buildPayload assembles the fields for each supported combination explicitly,
// as the Rust SDK's match does — so what each channel sends is visible in one
// place, and a new channel cannot silently inherit another's fields.
func buildPayload(in SendInput) (map[string]any, error) {
	p := map[string]any{
		"message_type": string(in.Type),
		"channel":      string(in.Channel),
		"template":     string(in.Template),
		"identity":     in.Identity,
		"to":           in.Payload.To,
	}
	optional := func(key, value string) {
		if value != "" {
			p[key] = value
		}
	}

	switch {
	case in.Channel == ChannelEmail && in.Type == MessageTypeVerification && in.Template == TemplateOtp,
		in.Channel == ChannelWhatsApp && in.Type == MessageTypeVerification && in.Template == TemplateOtp:
		p["otp"] = in.Payload.Otp

	case in.Channel == ChannelEmail && in.Type == MessageTypeTransactional && in.Template == TemplateCustom:
		p["subject"] = in.Payload.Subject
		p["body"] = in.Payload.Body

	case in.Channel == ChannelWhatsApp && in.Type == MessageTypeTransactional && in.Template == TemplateCustom:
		// Subject is optional here: the server sends it as a bold first line.
		optional("subject", in.Payload.Subject)
		p["body"] = in.Payload.Body

	case in.Channel == ChannelPush && in.Type == MessageTypeNotification && in.Template == TemplateNotification:
		p["body"] = in.Payload.Body
		optional("title", in.Payload.Title)
		optional("image", in.Payload.Image)
		optional("link", in.Payload.Link)
		optional("device_key", in.Payload.DeviceKey)
		if len(in.Payload.Data) > 0 {
			p["data"] = in.Payload.Data
		}

	default:
		return nil, &Error{
			Kind:    KindValidation,
			Message: "Unsupported combination: " + string(in.Type) + "." + string(in.Channel) + "." + string(in.Template),
		}
	}

	return p, nil
}
