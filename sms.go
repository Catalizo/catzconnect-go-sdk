package catzconnect

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
)

// SMSMaxSegments is the longest body the server accepts, in segments
// (~1,530 GSM-7 or ~670 UCS-2 characters).
const SMSMaxSegments = 10

// SMS body encodings, as the server reports them.
const (
	SMSEncodingGSM7 = "gsm7"
	SMSEncodingUCS2 = "ucs2"
)

// SMSAnalysis is how the server will encode an SMS body.
type SMSAnalysis struct {
	Encoding string // "gsm7" or "ucs2"
	Units    int    // septets (GSM-7) or UTF-16 code units (UCS-2)
	Segments int    // billable parts
	Chars    int    // characters as a person would count them
}

// SMSResult is the data of a successful SMS send. The message is queued,
// not yet delivered; webhook events (sms.sent, sms.delivered, sms.failed,
// sms.unknown — the provider was called but did not answer, so it is not
// resent) carry MessageID.
type SMSResult struct {
	MessageID string `json:"message_id"`
	Segments  int    `json:"segments"`
	Encoding  string `json:"encoding"`
	// Message is the server's message ("Success" etc.).
	Message string `json:"-"`
}

var e164Formatting = strings.NewReplacer(" ", "", "-", "", ".", "", "(", "", ")", "")
var e164Re = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// NormalizeE164 normalises a phone number the way the server does: spaces,
// dashes, dots and brackets are removed, and the result must be + and 8–15
// digits not starting with 0. ok is false when it is not E.164.
func NormalizeE164(raw string) (e164 string, ok bool) {
	cleaned := e164Formatting.Replace(strings.TrimSpace(raw))
	if e164Re.MatchString(cleaned) {
		return cleaned, true
	}
	return "", false
}

func validateE164(phone string) (string, error) {
	n, ok := NormalizeE164(phone)
	if !ok {
		return "", fmt.Errorf("'%s' is not an E.164 phone number — use + and the country code, e.g. +919876543210", strings.TrimSpace(phone))
	}
	return n, nil
}

var senderRe = regexp.MustCompile(`^[A-Za-z0-9 ]{2,11}$`)
var hasLetter = regexp.MustCompile(`[A-Za-z]`)
var dltRe = regexp.MustCompile(`^[0-9]{6,30}$`)

// IsValidSMSSender reports whether s is a sender the server accepts: an
// E.164 number, or 2–11 letters, digits and spaces with at least one letter.
func IsValidSMSSender(s string) bool {
	if strings.HasPrefix(s, "+") {
		n, ok := NormalizeE164(s)
		return ok && n == s
	}
	return senderRe.MatchString(s) && hasLetter.MatchString(s) &&
		!strings.HasPrefix(s, " ") && !strings.HasSuffix(s, " ")
}

// IsValidDLTID reports whether s is an India DLT template id (6–30 digits).
func IsValidDLTID(s string) bool { return dltRe.MatchString(s) }

// GSM 03.38 default alphabet (ESC excluded) and extension table (ESC + char).
const gsmBasic = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"
const gsmExtension = "^{}\\[~]|€\f"

func smsSegments(costs []int, total, single, multi int) int {
	if total <= single {
		return 1
	}
	parts, used := 1, 0
	for _, c := range costs {
		if used+c > multi {
			parts++
			used = 0
		}
		used += c
	}
	return parts
}

// AnalyzeSMS reports how the server will encode text and how many segments
// it takes: GSM-7 when every character is in the GSM alphabet (extension
// characters cost two septets), otherwise UCS-2. 160/153 septets or 70/67
// code units per part; a character is never split across parts.
func AnalyzeSMS(text string) SMSAnalysis {
	runes := []rune(text)
	costs := make([]int, len(runes))
	units, gsm := 0, true
	for i, r := range runes {
		if strings.ContainsRune(gsmBasic, r) {
			costs[i] = 1
		} else if strings.ContainsRune(gsmExtension, r) {
			costs[i] = 2
		} else {
			gsm = false
			break
		}
		units += costs[i]
	}
	if gsm {
		return SMSAnalysis{Encoding: SMSEncodingGSM7, Units: units, Segments: smsSegments(costs, units, 160, 153), Chars: len(runes)}
	}
	units = 0
	for i, r := range runes {
		costs[i] = len(utf16.Encode([]rune{r}))
		units += costs[i]
	}
	return SMSAnalysis{Encoding: SMSEncodingUCS2, Units: units, Segments: smsSegments(costs, units, 70, 67), Chars: len(runes)}
}

// validateSMSBody returns the server's error for a body it would refuse.
func validateSMSBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("The message is empty")
	}
	for _, r := range body {
		if unicode.Is(unicode.Cc, r) && r != '\n' && r != '\r' && r != '\t' {
			return fmt.Errorf("The message contains control characters")
		}
	}
	a := AnalyzeSMS(body)
	if a.Segments > SMSMaxSegments {
		chars := 670
		if a.Encoding == SMSEncodingGSM7 {
			chars = 1530
		}
		return fmt.Errorf("The message is %d segments long; the limit is %d (%d characters in %s)",
			a.Segments, SMSMaxSegments, chars, strings.ToUpper(a.Encoding))
	}
	return nil
}

var smsOtp = regexp.MustCompile(`^[A-Za-z0-9]{1,15}$`)

// verifySMS applies the server's checks for channel SMS: Otp with
// Verification and Otp; Custom with Transactional or Promotional and Body;
// any other name is an SMS template from the panel, filled from Data.
// Template names match case-insensitively, as on the server. Identity is
// ignored.
func verifySMS(input SendInput) error {
	p := input.Payload
	if strings.TrimSpace(p.To) == "" {
		return fmt.Errorf("Missing 'to' in payload — the recipient's number in E.164, e.g. +919876543210")
	}
	to, err := validateE164(p.To)
	if err != nil {
		return err
	}

	switch input.Type {
	case MessageTypeVerification, MessageTypeTransactional, MessageTypePromotional:
	default:
		return fmt.Errorf("SMS supports type Verification (template Otp), Transactional or Promotional — not %s", input.Type)
	}

	gateway := strings.TrimSpace(p.Gateway)
	if sender := strings.TrimSpace(p.Sender); sender != "" {
		if gateway == "" {
			return fmt.Errorf("'sender' can only be used together with 'gateway'")
		}
		if !IsValidSMSSender(sender) {
			return fmt.Errorf("'sender' must be an E.164 number or a 2–11 character sender ID")
		}
	}

	dlt := strings.TrimSpace(p.DLTTemplateID)
	if dlt != "" && !IsValidDLTID(dlt) {
		return fmt.Errorf("'dlt_template_id' must be the numeric DLT template ID")
	}

	name := strings.TrimSpace(string(input.Template))
	switch {
	case name == "":
		return fmt.Errorf("Missing 'template' — Otp, Custom, or the name of an SMS template")

	case strings.EqualFold(name, "otp"):
		if input.Type != MessageTypeVerification {
			return fmt.Errorf("template Otp goes with type Verification")
		}
		otp := strings.TrimSpace(p.Otp)
		if otp == "" {
			return fmt.Errorf("Missing 'otp' in payload")
		}
		if !smsOtp.MatchString(otp) {
			return fmt.Errorf("'otp' must be up to 15 letters or digits")
		}

	case strings.EqualFold(name, "custom"):
		if input.Type == MessageTypeVerification {
			return fmt.Errorf("type Verification goes with template Otp; use Transactional for Custom")
		}
		body := strings.TrimSpace(p.Body)
		if body == "" {
			return fmt.Errorf("Missing 'body' in payload")
		}
		if err := validateSMSBody(body); err != nil {
			return err
		}
		// India needs a DLT template id; a Custom body can only get it from the payload.
		if strings.HasPrefix(to, "+91") && dlt == "" {
			return fmt.Errorf("India (+91) requires a DLT template ID: pass 'dlt_template_id' for Custom messages")
		}
	}
	// A panel SMS template: the server renders it from Data and checks its length.
	return nil
}

// buildSMSPayload lists exactly the fields the server reads for SMS.
func buildSMSPayload(in SendInput) map[string]any {
	p := map[string]any{
		"message_type": string(in.Type),
		"channel":      string(ChannelSMS),
		"template":     string(in.Template),
		"identity":     in.Identity, // required by the server, ignored for SMS
		"to":           in.Payload.To,
	}
	for k, v := range map[string]string{
		"otp":             in.Payload.Otp,
		"body":            in.Payload.Body,
		"gateway":         in.Payload.Gateway,
		"sender":          in.Payload.Sender,
		"dlt_template_id": in.Payload.DLTTemplateID,
	} {
		if strings.TrimSpace(v) != "" {
			p[k] = v
		}
	}
	if len(in.Payload.Data) > 0 {
		p["data"] = in.Payload.Data
	}
	return p
}

// ParseSMSResult reads the {message_id, segments, encoding} data out of the
// response Send returned for an SMS.
func ParseSMSResult(res map[string]any) (*SMSResult, error) {
	data, _ := res["data"].(map[string]any)
	id, _ := data["message_id"].(string)
	if id == "" {
		return nil, &Error{Kind: KindJSON, Message: "response has no data.message_id"}
	}
	out := &SMSResult{MessageID: id}
	if n, ok := data["segments"].(float64); ok {
		out.Segments = int(n)
	}
	out.Encoding, _ = data["encoding"].(string)
	out.Message, _ = res["message"].(string)
	return out, nil
}

// SendSMS sends an SMS (input.Channel may be left empty) and returns the
// queued message's id, segments and encoding.
func (c *CatzConnect) SendSMS(input SendInput, env *EnvValues) (*SMSResult, error) {
	if input.Channel == "" {
		input.Channel = ChannelSMS
	}
	if input.Channel != ChannelSMS {
		return nil, &Error{Kind: KindValidation, Message: "SendSMS is for the SMS channel, not " + string(input.Channel)}
	}
	res, err := c.Send(input, env)
	if err != nil {
		return nil, err
	}
	return ParseSMSResult(res)
}

// SendSMS sends an SMS using the package-level client.
func SendSMS(input SendInput, env *EnvValues) (*SMSResult, error) {
	return Default.SendSMS(input, env)
}
