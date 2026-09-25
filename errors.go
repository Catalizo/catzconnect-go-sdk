package catzconnect

import "fmt"

// ErrorKind classifies an *Error. It mirrors the variants of the Rust SDK's
// CatzError, so the two SDKs report the same failures the same way.
type ErrorKind int

const (
	KindMissingEnv ErrorKind = iota + 1
	KindInvalidKeyLength
	KindBase64
	KindJSON
	KindEncryption
	KindHTTP
	KindAPI
	KindValidation
)

// Error is every error Send returns. Use errors.Is with the sentinels below to
// branch on the kind, or errors.As to read Status and Body from an API error:
//
//	var ce *catzconnect.Error
//	if errors.As(err, &ce) && ce.Kind == catzconnect.KindAPI {
//	    log.Println(ce.Status, ce.Body)
//	}
type Error struct {
	Kind    ErrorKind
	Message string
	// Status and Body are set for KindAPI: the server answered with a
	// non-2xx status, and Body is what it said.
	Status int
	Body   string
	// Err is the underlying cause, when there is one.
	Err error
}

func (e *Error) Error() string {
	switch e.Kind {
	case KindMissingEnv:
		return "Missing environment variable: " + e.Message
	case KindInvalidKeyLength:
		return "Invalid key length — " + e.Message
	case KindBase64:
		return "Base64 decode error: " + e.Message
	case KindJSON:
		return "JSON serialization error: " + e.Message
	case KindEncryption:
		return "Encryption failed"
	case KindHTTP:
		return "HTTP error: " + e.Message
	case KindAPI:
		return fmt.Sprintf("API error (%d): %s", e.Status, e.Body)
	case KindValidation:
		return "Validation error: " + e.Message
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

// Is matches on Kind alone, so errors.Is(err, ErrAPI) is true for any API
// error whatever its status.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Kind == e.Kind && t.Message == "" && t.Status == 0
}

// Sentinels for errors.Is.
var (
	ErrMissingEnv       = &Error{Kind: KindMissingEnv}
	ErrInvalidKeyLength = &Error{Kind: KindInvalidKeyLength}
	ErrBase64           = &Error{Kind: KindBase64}
	ErrJSON             = &Error{Kind: KindJSON}
	ErrEncryption       = &Error{Kind: KindEncryption}
	ErrHTTP             = &Error{Kind: KindHTTP}
	ErrAPI              = &Error{Kind: KindAPI}
	ErrValidation       = &Error{Kind: KindValidation}
)
