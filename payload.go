package catzconnect

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var digitsOnly = regexp.MustCompile(`^\d+$`)

// Meta's authentication templates take up to 15 letters or digits.
var waOtp = regexp.MustCompile(`^[A-Za-z0-9]{1,15}$`)

// verifyPayload validates the input for the supported operations. It mirrors
// the TypeScript SDK's checks, including the exact error messages. The
// TypeScript `typeof x !== "string"` guards are omitted here because Go's
// type system already guarantees these fields are strings.
func verifyPayload(input SendInput) error {
	if input.Payload.Tracking != nil && input.Channel != ChannelEmail {
		return errors.New("'tracking' applies to the Email channel only")
	}
	if err := verifyTopicAndPush(input); err != nil {
		return err
	}
	if input.Payload.WhatsApp != nil && input.Channel != ChannelWhatsApp {
		return errors.New("'whatsapp' applies to the WhatsApp channel only")
	}

	// WhatsApp media message: template Custom with WhatsApp.Media (any type).
	if input.Channel == ChannelWhatsApp && input.Template == TemplateCustom && input.Payload.WhatsApp != nil && input.Payload.WhatsApp.Media != nil {
		if input.Identity == "" {
			return errors.New("Missing 'identity'")
		}
		if input.Payload.To == "" {
			return errors.New("Missing 'to' in payload")
		}
		if err := validatePhone(input.Payload.To); err != nil {
			return err
		}
		return VerifyWhatsAppMedia(*input.Payload.WhatsApp.Media)
	}

	// WhatsApp approved template, sent by its Meta name.
	if input.Channel == ChannelWhatsApp && input.Template != TemplateOtp && input.Template != TemplateCustom {
		switch input.Type {
		case MessageTypeTransactional, MessageTypeVerification, MessageTypePromotional, MessageTypeNotification:
		default:
			return fmt.Errorf("Unsupported combination: %s.%s.%s", input.Type, input.Channel, input.Template)
		}
		if !waTemplateName.MatchString(string(input.Template)) {
			return errors.New("A WhatsApp template name is lower-case letters, digits and underscores (the name approved at Meta)")
		}
		if input.Identity == "" {
			return errors.New("Missing 'identity'")
		}
		if input.Payload.To == "" {
			return errors.New("Missing 'to' in payload")
		}
		if err := validatePhone(input.Payload.To); err != nil {
			return err
		}
		if input.Payload.Otp != "" && !waOtp.MatchString(input.Payload.Otp) {
			return errors.New("'otp' must be up to 15 letters or digits")
		}
		if input.Payload.WhatsApp != nil {
			if input.Payload.WhatsApp.Media != nil {
				return errors.New("'whatsapp.media' goes with template Custom; use 'whatsapp.header' for a template's media")
			}
			return VerifyWhatsAppOptions(*input.Payload.WhatsApp)
		}
		return nil
	}

	// Email · Verification · Otp
	if input.Channel == ChannelEmail && input.Type == MessageTypeVerification && input.Template == TemplateOtp {
		if input.Identity == "" {
			return errors.New("Missing 'identity'")
		}
		if input.Payload.To == "" {
			return errors.New("Missing 'to' in payload")
		}
		if input.Payload.Otp == "" {
			return errors.New("Missing 'otp' in payload")
		}

		if err := validateEmail(input.Payload.To); err != nil {
			return err
		}

		if !digitsOnly.MatchString(input.Payload.Otp) {
			return errors.New("'otp' must contain only digits")
		}
		if len(input.Payload.Otp) != 6 {
			return errors.New("'otp' must be exactly 6 digits")
		}

		return nil
	}

	// Email · Transactional · Custom
	if input.Channel == ChannelEmail && input.Type == MessageTypeTransactional && input.Template == TemplateCustom {
		if input.Identity == "" {
			return errors.New("Missing 'identity'")
		}
		if input.Payload.To == "" {
			return errors.New("Missing 'to' in payload")
		}
		if input.Payload.Subject == "" {
			return errors.New("Missing 'subject' in payload")
		}
		if input.Payload.Body == "" {
			return errors.New("Missing 'body' in payload")
		}

		if err := validateEmail(input.Payload.To); err != nil {
			return err
		}

		return nil
	}

	// WhatsApp · Verification · Otp
	if input.Channel == ChannelWhatsApp && input.Type == MessageTypeVerification && input.Template == TemplateOtp {
		if input.Identity == "" {
			return errors.New("Missing 'identity'")
		}
		if input.Payload.To == "" {
			return errors.New("Missing 'to' in payload")
		}
		if err := validatePhone(input.Payload.To); err != nil {
			return err
		}
		if input.Payload.Otp == "" {
			return errors.New("Missing 'otp' in payload")
		}
		if !waOtp.MatchString(input.Payload.Otp) {
			return errors.New("'otp' must be up to 15 letters or digits")
		}
		return nil
	}

	// WhatsApp · Transactional · Custom — subject optional, sent as a bold line.
	if input.Channel == ChannelWhatsApp && input.Type == MessageTypeTransactional && input.Template == TemplateCustom {
		if input.Identity == "" {
			return errors.New("Missing 'identity'")
		}
		if input.Payload.To == "" {
			return errors.New("Missing 'to' in payload")
		}
		if err := validatePhone(input.Payload.To); err != nil {
			return err
		}
		if input.Payload.Body == "" {
			return errors.New("Missing 'body' in payload")
		}
		length := len([]rune(input.Payload.Body))
		if input.Payload.Subject != "" {
			length += len([]rune(input.Payload.Subject)) + 6
		}
		if length > 4096 {
			return errors.New("WhatsApp messages are limited to 4096 characters")
		}
		return nil
	}

	// Push · Notification · Notification
	if input.Channel == ChannelPush && input.Type == MessageTypeNotification && input.Template == TemplateNotification {
		if input.Identity == "" {
			return errors.New("Missing 'identity' — the Firebase project ID")
		}
		hasTo := input.Payload.To != ""
		hasUser := input.Payload.ExternalUserID != ""
		many := input.Payload.Push != nil && (input.Payload.Push.Broadcast || input.Payload.Push.Topic != "")
		if hasTo && hasUser {
			return errors.New("Give either 'to' (one device token) or 'external_user_id' (a user's registered devices), not both")
		}
		if !hasTo && !hasUser && !many {
			return errors.New("Missing 'to' in payload — the device's FCM registration token — or 'external_user_id' for a user's registered devices")
		}
		if hasTo && strings.Contains(input.Payload.To, "@") {
			return errors.New("'to' must be an FCM registration token, not an email address")
		}
		if hasUser {
			if len([]rune(input.Payload.ExternalUserID)) > 128 {
				return errors.New("'external_user_id' must be a string of up to 128 characters")
			}
			if input.Payload.DeviceKey != "" {
				return errors.New("'device_key' cannot be used with 'external_user_id' — each registered device's own key is used")
			}
		}
		if input.Payload.Body == "" {
			return errors.New("Missing 'body' in payload")
		}
		for name, v := range map[string]string{"image": input.Payload.Image, "link": input.Payload.Link} {
			if v != "" && !strings.HasPrefix(v, "https://") {
				return fmt.Errorf("'%s' must be an https:// URL", name)
			}
		}
		return nil
	}

	// SMS · Otp, Custom, or an SMS template from the panel.
	if input.Channel == ChannelSMS {
		return verifySMS(input)
	}

	// Voice · a phone call: Otp, Custom, or a Voice template from the panel.
	if input.Channel == ChannelVoice {
		return verifyVoice(input)
	}

	// Email · a template created in the panel, sent by name. The server
	// fills its {{variables}} from Payload.Data.
	if input.Channel == ChannelEmail && input.Template != "" && !isBuiltinTemplate(input.Template) {
		if input.Identity == "" {
			return errors.New("Missing 'identity'")
		}
		if input.Payload.To == "" {
			return errors.New("Missing 'to' in payload")
		}
		if err := validateEmail(input.Payload.To); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("Unsupported combination: %s.%s.%s", input.Type, input.Channel, input.Template)
}

var (
	// EmailTopicKey matches subscription topic keys (Email `topic`).
	EmailTopicKey = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)
	// PushTopicName matches push topic names: 1–64 of A-Z a-z 0-9 _ . ~ -.
	PushTopicName = regexp.MustCompile(`^[A-Za-z0-9_.~-]{1,64}$`)
	rfc3339Zone   = regexp.MustCompile(`(?i)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2})$`)
)

// PushSendAt formats a time for PushOptions.SendAt (RFC 3339, UTC).
func PushSendAt(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// verifyTopicAndPush checks `topic` (Email) and `push` (Push options).
func verifyTopicAndPush(input SendInput) error {
	if input.Payload.Topic != "" {
		if input.Channel != ChannelEmail {
			return errors.New("'topic' applies to the Email channel only — for push topics use push.topic")
		}
		if !EmailTopicKey.MatchString(input.Payload.Topic) {
			return errors.New("'topic' must be a topic key: lower-case letters, digits, _ and -, up to 40 characters")
		}
	}
	p := input.Payload.Push
	if p == nil {
		return nil
	}
	if input.Channel != ChannelPush {
		return errors.New("'push' applies to the Push channel only")
	}
	if p.Broadcast && p.Topic != "" {
		return errors.New("Give push.broadcast or push.topic, not both")
	}
	if p.Topic != "" && !PushTopicName.MatchString(p.Topic) {
		return errors.New("'push.topic' must be 1–64 characters of letters, digits, _ . ~ -")
	}
	if p.SendAt != "" {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(p.SendAt)); err != nil || !rfc3339Zone.MatchString(strings.TrimSpace(p.SendAt)) {
			return errors.New("'push.send_at' must be an RFC 3339 time with a time zone, e.g. 2026-11-02T09:00:00Z")
		}
	}
	if p.Broadcast || p.Topic != "" {
		if input.Payload.To != "" || input.Payload.ExternalUserID != "" {
			return errors.New("push.broadcast and push.topic send to many devices: leave 'to' and 'external_user_id' out")
		}
		if input.Payload.DeviceKey != "" {
			return errors.New("'device_key' is only for one device token — registered devices use their own keys")
		}
	}
	return nil
}
