package catzconnect

import (
	"errors"
	"fmt"
	"regexp"
)

var digitsOnly = regexp.MustCompile(`^\d+$`)

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

	return fmt.Errorf("Unsupported combination: %s.%s.%s", input.Type, input.Channel, input.Template)
}
