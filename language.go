package catzconnect

import (
	"regexp"
	"strings"
)

// SupportedLanguages are the languages of the built-in (default)
// Otp/Verification templates, as SendInput.Language accepts them.
var SupportedLanguages = []string{"en", "de", "ja", "ar", "hi"}

var languageTagRe = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-_][A-Za-z0-9]+)*$`)

// NormalizeLanguage reduces a language for SendInput.Language to its BCP-47
// primary subtag: "de", "DE", "de-DE" and "de_DE" all become "de". It
// returns a validation *Error when the language is not one of en, de, ja,
// ar, hi.
func NormalizeLanguage(language string) (string, error) {
	tag := strings.TrimSpace(language)
	primary := tag
	if i := strings.IndexAny(tag, "-_"); i >= 0 {
		primary = tag[:i]
	}
	primary = strings.ToLower(primary)
	if languageTagRe.MatchString(tag) {
		for _, l := range SupportedLanguages {
			if primary == l {
				return l, nil
			}
		}
	}
	return "", &Error{
		Kind:    KindValidation,
		Message: `Unsupported 'language' "` + language + `": use one of en, de, ja, ar, hi (or a BCP-47 tag such as de-DE)`,
	}
}
