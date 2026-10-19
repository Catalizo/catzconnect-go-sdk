package catzconnect

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// VoiceMaxChars is the longest Custom text a phone call speaks.
const VoiceMaxChars = 1000

// VoiceMaxRepeat is the most times a phone call repeats its message.
const VoiceMaxRepeat = 3

var voiceLanguageRe = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8}){0,2}$`)
var voiceNameRe = regexp.MustCompile(`^[A-Za-z0-9._:\- ]{1,64}$`)

// IsValidVoiceLanguage reports whether s is a language tag such as en-US,
// hi-IN, es-419 or fr.
func IsValidVoiceLanguage(s string) bool { return voiceLanguageRe.MatchString(s) }

// IsValidVoiceName reports whether s is "female", "male" or a provider voice
// name such as "Polly.Joanna" or "en-US-Wavenet-F".
func IsValidVoiceName(s string) bool {
	return voiceNameRe.MatchString(s) && strings.TrimSpace(s) == s
}

// ValidateVoiceText returns the server's error for a Custom text it would
// refuse: empty, control characters, or over 1,000 characters.
func ValidateVoiceText(text string) error {
	t := strings.TrimSpace(text)
	if t == "" {
		return fmt.Errorf("The text to speak is empty")
	}
	for _, r := range t {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return fmt.Errorf("The text to speak contains control characters")
		}
	}
	if n := utf8.RuneCountInString(t); n > VoiceMaxChars {
		return fmt.Errorf("The text to speak is %d characters; the limit is %d", n, VoiceMaxChars)
	}
	return nil
}

// ParseVoiceRepeat reads Data["repeat"] as the server does: a whole number 1–3.
func ParseVoiceRepeat(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > VoiceMaxRepeat {
		return 0, fmt.Errorf("'data.repeat' must be a whole number from 1 to 3")
	}
	return n, nil
}

// canonicalChannel maps "voice" in any casing to ChannelVoice; other
// channels are passed through unchanged, as before.
func canonicalChannel(c Channel) Channel {
	if strings.EqualFold(strings.TrimSpace(string(c)), "voice") {
		return ChannelVoice
	}
	return c
}

// verifyVoice applies the server's checks for channel Voice: an E.164 To;
// template Otp with Otp (up to 15 letters or digits, read out one by one),
// Custom with Body (≤ 1,000 characters), or the name of a Voice template from
// the panel filled from Data. Data "language", "voice" and "repeat" are
// optional. Identity is ignored.
func verifyVoice(input SendInput) error {
	p := input.Payload
	if strings.TrimSpace(p.To) == "" {
		return fmt.Errorf("Missing 'to' in payload — the number to call in E.164, e.g. +14155550100")
	}
	if _, err := validateE164(p.To); err != nil {
		return err
	}

	switch input.Type {
	case MessageTypeVerification, MessageTypeTransactional, MessageTypeNotification, MessageTypePromotional:
	default:
		return fmt.Errorf("Voice supports type Verification, Transactional, Notification or Promotional — not %s", input.Type)
	}

	for k, v := range p.Data {
		switch k {
		case "repeat":
			if _, err := ParseVoiceRepeat(v); err != nil {
				return err
			}
		case "language":
			if !IsValidVoiceLanguage(v) {
				return fmt.Errorf("'data.language' must be a language tag such as en-US")
			}
		case "voice":
			if !IsValidVoiceName(v) {
				return fmt.Errorf("'data.voice' must be female, male or a provider voice name")
			}
		}
	}

	name := strings.TrimSpace(string(input.Template))
	switch {
	case name == "":
		return fmt.Errorf("Missing 'template' — Otp, Custom, or the name of a Voice template")
	case strings.EqualFold(name, "otp"):
		otp := strings.TrimSpace(p.Otp)
		if otp == "" {
			return fmt.Errorf("Missing 'otp' in payload")
		}
		if !smsOtp.MatchString(otp) {
			return fmt.Errorf("'otp' must be up to 15 letters or digits")
		}
	case strings.EqualFold(name, "custom"):
		if strings.TrimSpace(p.Body) == "" {
			return fmt.Errorf("Missing 'body' in payload — the text to speak")
		}
		if err := ValidateVoiceText(p.Body); err != nil {
			return err
		}
	}
	// A panel Voice template: the server renders it from Data.
	return nil
}

// buildVoicePayload lists exactly the fields the server reads for Voice.
func buildVoicePayload(in SendInput) map[string]any {
	to, _ := NormalizeE164(in.Payload.To)
	tpl := strings.TrimSpace(string(in.Template))
	switch {
	case strings.EqualFold(tpl, "otp"):
		tpl = string(TemplateOtp)
	case strings.EqualFold(tpl, "custom"):
		tpl = string(TemplateCustom)
	}
	p := map[string]any{
		"message_type": string(in.Type),
		"channel":      string(ChannelVoice),
		"template":     tpl,
		"identity":     in.Identity, // required by the server, ignored for Voice
		"to":           to,
	}
	for k, v := range map[string]string{
		"otp":      in.Payload.Otp,
		"body":     in.Payload.Body,
		"provider": in.Payload.Provider,
	} {
		if t := strings.TrimSpace(v); t != "" {
			p[k] = t
		}
	}
	if len(in.Payload.Data) > 0 {
		data := make(map[string]string, len(in.Payload.Data))
		for k, v := range in.Payload.Data {
			if k == "repeat" {
				if n, err := ParseVoiceRepeat(v); err == nil {
					v = strconv.Itoa(n)
				}
			}
			data[k] = v
		}
		p["data"] = data
	}
	return p
}

// VoiceResult is the data of a successful Voice send: the phone call is
// queued, not yet placed. voice.* webhook events carry MessageID.
type VoiceResult struct {
	MessageID string
	// Message is the server's message.
	Message string
	// Data is the whole data object, for fields added later.
	Data map[string]any
}

// ParseVoiceResult reads data.message_id out of the response Send returned
// for a Voice send.
func ParseVoiceResult(res map[string]any) (*VoiceResult, error) {
	data, _ := res["data"].(map[string]any)
	id, _ := data["message_id"].(string)
	if id == "" {
		return nil, &Error{Kind: KindJSON, Message: "response has no data.message_id"}
	}
	out := &VoiceResult{MessageID: id, Data: data}
	out.Message, _ = res["message"].(string)
	return out, nil
}

// SendVoice places a phone call (input.Channel may be left empty) and
// returns the queued call's id.
func (c *CatzConnect) SendVoice(input SendInput, env *EnvValues) (*VoiceResult, error) {
	if input.Channel == "" {
		input.Channel = ChannelVoice
	}
	if canonicalChannel(input.Channel) != ChannelVoice {
		return nil, &Error{Kind: KindValidation, Message: "SendVoice is for the Voice channel, not " + string(input.Channel)}
	}
	res, err := c.Send(input, env)
	if err != nil {
		return nil, err
	}
	return ParseVoiceResult(res)
}

// SendVoice places a phone call using the package-level client.
func SendVoice(input SendInput, env *EnvValues) (*VoiceResult, error) {
	return Default.SendVoice(input, env)
}
