package catzconnect

import (
	"fmt"
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// validateEmail returns an error if email is not a valid address.
func validateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("Invalid email: %s", email)
	}
	return nil
}

var phoneChars = regexp.MustCompile(`^[\d\s()+\-]+$`)
var nonDigit = regexp.MustCompile(`\D`)

// validatePhone rejects what can never be a WhatsApp recipient: an email
// address, or a digit count no real number has. The server normalises the
// rest and applies the number's default country code to national numbers.
func validatePhone(phone string) error {
	if strings.Contains(phone, "@") {
		return fmt.Errorf("WhatsApp messages go to phone numbers, not email addresses: %s", phone)
	}
	if !phoneChars.MatchString(phone) {
		return fmt.Errorf("Invalid phone number: %s", phone)
	}
	digits := nonDigit.ReplaceAllString(phone, "")
	if len(digits) < 7 || len(digits) > 15 {
		return fmt.Errorf("Invalid phone number: %s", phone)
	}
	return nil
}
