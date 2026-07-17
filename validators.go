package catzconnect

import (
	"fmt"
	"regexp"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// validateEmail returns an error if email is not a valid address.
func validateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("Invalid email: %s", email)
	}
	return nil
}
