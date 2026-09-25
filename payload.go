package catzconnect

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var digitsOnly = regexp.MustCompile(`^\d+$`)

// Meta's authentication templates take up to 15 letters or digits.
var waOtp = regexp.MustCompile(`^[A-Za-z0-9]{1,15}$`)

// verifyPayload validates the input for the supported operations. It mirrors
// the TypeScript SDK's checks, including the exact error messages. The
// TypeScript `typeof x !== "string"` guards are omitted here because Go's
// type system already guarantees these fields are strings.
func verifyPayload(input SendInput) error {
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
		if input.Payload.To == "" {
			return errors.New("Missing 'to' in payload — the device's FCM registration token")
		}
		if strings.Contains(input.Payload.To, "@") {
			return errors.New("'to' must be an FCM registration token, not an email address")
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

	return fmt.Errorf("Unsupported combination: %s.%s.%s", input.Type, input.Channel, input.Template)
}
