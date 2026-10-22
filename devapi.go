package catzconnect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Machine codes the API sends in Error.Code. Branch on these, not on the
// (translated) message.
const (
	CodeOrganizationSuspended            = "organization_suspended"
	CodeSendingPaused                    = "sending_paused"
	CodeFeatureNotInPlan                 = "feature_not_in_plan"
	CodePlanRequired                     = "plan_required"
	CodeChannelLimitReached              = "channel_limit_reached"
	CodeDailyLimitReached                = "daily_limit_reached"
	CodeMonthlyLimitReached              = "monthly_limit_reached"
	CodeMinutesCapReached                = "minutes_cap_reached"
	CodeKeyExpired                       = "key_expired"
	CodeIPNotAllowed                     = "ip_not_allowed"
	CodeKeyRateLimited                   = "key_rate_limited"
	CodeKeySendLimit                     = "key_send_limit"
	CodeNotAllowedForAppKeys             = "not_allowed_for_app_keys"
	CodeNotAllowedForTestKeys            = "not_allowed_for_test_keys"
	CodeManagementScopeRequired          = "management_scope_required"
	CodeWhatsAppInvalidTemplate          = "whatsapp_invalid_template"
	CodeWhatsAppTemplateNotApproved      = "whatsapp_template_not_approved"
	CodeWhatsAppTemplateLanguageRequired = "whatsapp_template_language_required"
	CodeWhatsAppInvalidTemplateParams    = "whatsapp_invalid_template_params"
	CodeWhatsAppInvalidMedia             = "whatsapp_invalid_media"
	CodeWhatsAppWindowClosed             = "whatsapp_window_closed"
	CodeIdempotencyConflict              = "idempotency_conflict"
	CodeIdempotencyInProgress            = "idempotency_in_progress"
	CodeInvalidIdempotencyKey            = "invalid_idempotency_key"
	CodeInvalidReference                 = "invalid_reference"
	CodeMessageNotFound                  = "message_not_found"
	CodeOtpInvalid                       = "otp_invalid"
	CodeOtpExpired                       = "otp_expired"
	CodeOtpNotFound                      = "otp_not_found"
	CodeOtpAlreadyVerified               = "otp_already_verified"
	CodeOtpAttemptsExceeded              = "otp_attempts_exceeded"
	CodeOtpResendCooldown                = "otp_resend_cooldown"
	CodeOtpSendLimit                     = "otp_send_limit"
	CodeUnsupportedChannel               = "unsupported_channel"
	CodeUnknownOperation                 = "unknown_operation"
	CodeInvalidRequest                   = "invalid_request"
	CodeNotFound                         = "not_found"
	CodeConflict                         = "conflict"
	CodeInvalidTopic                     = "invalid_topic"
	CodeTopicEmailOnly                   = "topic_email_only"
	CodeRecipientSuppressed              = "recipient_suppressed"
	CodeWhatsAppOptedOut                 = "whatsapp_opted_out"
	CodeNotCancelable                    = "not_cancelable"
	CodeNotResendable                    = "not_resendable"
	CodeContentGone                      = "content_gone"
)

// IsRetryableCode reports refusals worth retrying later with the same request.
func IsRetryableCode(code string) bool {
	switch code {
	case CodeSendingPaused, CodeKeyRateLimited, CodeKeySendLimit, CodeIdempotencyInProgress, CodeOtpResendCooldown:
		return true
	}
	return false
}

var (
	waTemplateName = regexp.MustCompile(`^[a-z0-9_]{1,512}$`)
	waLanguage     = regexp.MustCompile(`^[A-Za-z]{2,3}(_[A-Za-z0-9]{2,4})?$`)
	mediaID        = regexp.MustCompile(`^\d{1,64}$`)
	idemKey        = regexp.MustCompile(`^[\x21-\x7e]{1,255}$`)
	uuidRe         = regexp.MustCompile(`^(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// ValidateIdempotencyKey: 1–255 visible ASCII characters, no spaces.
func ValidateIdempotencyKey(k string) error {
	if !idemKey.MatchString(strings.TrimSpace(k)) {
		return &Error{Kind: KindValidation, Message: "'idempotency_key' must be 1–255 visible ASCII characters without spaces"}
	}
	return nil
}

// ValidateReference returns the trimmed reference ("" for blank): up to 128
// characters without control characters.
func ValidateReference(r string) (string, error) {
	t := strings.TrimSpace(r)
	if utf8.RuneCountInString(t) > 128 || strings.IndexFunc(t, unicode.IsControl) >= 0 {
		return "", &Error{Kind: KindValidation, Message: "'reference' must be up to 128 characters without line breaks or control characters"}
	}
	return t, nil
}

func checkMediaRef(kind, link, id, filename, where string) error {
	if (link == "") == (id == "") {
		return fmt.Errorf("%s: give exactly one of 'link' (https) or 'id' (Meta media id)", where)
	}
	if link != "" && (!strings.HasPrefix(link, "https://") || len(link) > 2048 || strings.ContainsAny(link, " \t\n")) {
		return fmt.Errorf("%s.link must be an https:// URL", where)
	}
	if id != "" && !mediaID.MatchString(id) {
		return fmt.Errorf("%s.id must be a Meta media id (digits)", where)
	}
	if filename != "" {
		if kind != "document" {
			return fmt.Errorf("%s.filename is for documents only", where)
		}
		if utf8.RuneCountInString(filename) > 240 || strings.ContainsAny(filename, `/\`) {
			return fmt.Errorf("%s.filename must be up to 240 characters without slashes", where)
		}
	}
	return nil
}

// VerifyWhatsAppMedia checks a media message as the server does.
func VerifyWhatsAppMedia(m WhatsAppMedia) error {
	switch m.Type {
	case "image", "document", "video", "audio":
	default:
		return errors.New("'whatsapp.media.type' must be image, document, video or audio")
	}
	if err := checkMediaRef(m.Type, m.Link, m.ID, m.Filename, "'whatsapp.media'"); err != nil {
		return err
	}
	if m.Caption != "" {
		if m.Type == "audio" {
			return errors.New("'whatsapp.media.caption' is not allowed for audio")
		}
		if utf8.RuneCountInString(m.Caption) > 1024 {
			return errors.New("'whatsapp.media.caption' is at most 1024 characters")
		}
	}
	return nil
}

func checkParam(v, where string) error {
	if strings.TrimSpace(v) == "" || utf8.RuneCountInString(v) > 1024 || strings.ContainsAny(v, "\n\t") || strings.Contains(v, "     ") {
		return fmt.Errorf("%s must be non-empty text of up to 1024 characters without new lines, tabs or 5+ spaces in a row", where)
	}
	return nil
}

// VerifyWhatsAppOptions checks approved-template parameters.
func VerifyWhatsAppOptions(o WhatsAppOptions) error {
	if o.Language != "" && !waLanguage.MatchString(o.Language) {
		return errors.New("'whatsapp.language' must be a Meta language code such as en, en_US or es_419")
	}
	if len(o.Body) > 0 && len(o.NamedBody) > 0 {
		return errors.New("set Body or NamedBody, not both")
	}
	if h := o.Header; h != nil {
		switch h.Type {
		case "text":
			if strings.TrimSpace(h.Text) == "" || utf8.RuneCountInString(h.Text) > 60 {
				return errors.New("'whatsapp.header.text' must be 1–60 characters")
			}
		case "image", "video", "document":
			if err := checkMediaRef(h.Type, h.Link, h.ID, h.Filename, "'whatsapp.header'"); err != nil {
				return err
			}
		case "location":
			if h.Latitude == nil || h.Longitude == nil {
				return errors.New("'whatsapp.header' location needs latitude and longitude")
			}
		default:
			return errors.New("'whatsapp.header.type' must be text, image, video, document or location")
		}
	}
	for i, v := range o.Body {
		if err := checkParam(v, fmt.Sprintf("'whatsapp.body[%d]'", i)); err != nil {
			return err
		}
	}
	for k, v := range o.NamedBody {
		if err := checkParam(v, "'whatsapp.body."+k+"'"); err != nil {
			return err
		}
	}
	seen := map[int]bool{}
	for _, b := range o.Buttons {
		if b.Index < 0 || b.Index > 9 {
			return errors.New("'whatsapp.buttons[].index' must be 0–9")
		}
		if seen[b.Index] {
			return fmt.Errorf("'whatsapp.buttons' lists index %d twice", b.Index)
		}
		seen[b.Index] = true
		switch b.Type {
		case "url":
			if b.Text == "" || strings.ContainsAny(b.Text, " \t\n") || len(b.Text) > 2000 {
				return errors.New("A url button needs 'text': the URL suffix without spaces")
			}
		case "quick_reply":
			if utf8.RuneCountInString(b.Payload) > 256 {
				return errors.New("A quick_reply 'payload' is at most 256 characters")
			}
		case "copy_code":
			if b.Code == "" || utf8.RuneCountInString(b.Code) > 20 {
				return errors.New("A copy_code button needs 'code' of up to 20 characters")
			}
		case "otp":
			if !waOtp.MatchString(b.Code) {
				return errors.New("An otp button needs 'code': up to 15 letters or digits")
			}
		default:
			return errors.New("'whatsapp.buttons[].type' must be url, quick_reply, copy_code or otp")
		}
	}
	if o.Media != nil {
		return VerifyWhatsAppMedia(*o.Media)
	}
	return nil
}

// ─── /sdk/* JSON calls ───────────────────────────────────────────────────

// sdkCall posts an encrypted body and returns `data` (or the whole answer).
func (c *CatzConnect) sdkCall(ctx context.Context, path string, body map[string]any, env *EnvValues) (map[string]any, error) {
	enc, err := encrypt(body, env)
	if err != nil {
		return nil, err
	}
	res, err := c.http.postCtx(ctx, path, enc, env)
	if err != nil {
		return nil, err
	}
	if ok, isBool := res["success"].(bool); isBool && !ok {
		b, _ := json.Marshal(res)
		return nil, apiError(200, b)
	}
	if d, ok := res["data"].(map[string]any); ok {
		return d, nil
	}
	return res, nil
}

func decodeInto(m map[string]any, out any) error {
	b, err := json.Marshal(m)
	if err != nil {
		return &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}
	if err := json.Unmarshal(b, out); err != nil {
		return &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}
	return nil
}

func validation(msg string) error { return &Error{Kind: KindValidation, Message: msg} }

// ─── message status ──────────────────────────────────────────────────────

// MessageEvent is one step of a message's timeline.
type MessageEvent struct {
	Type   string `json:"type"`
	At     string `json:"at"`
	Detail string `json:"detail,omitempty"`
}

// MessageStatus is a sent message as Messages().Get / List return it.
// Status is normalised: queued, deferred, sending, sent, delivered, read,
// ringing, answered, completed, failed, undelivered, suppressed, bounced,
// complained, unknown.
type MessageStatus struct {
	MessageID      string  `json:"message_id"`
	Channel        string  `json:"channel"`
	Status         string  `json:"status"`
	ProviderStatus string  `json:"provider_status"`
	To             string  `json:"to"`
	Template       *string `json:"template"`
	Reference      *string `json:"reference"`
	KeyID          *string `json:"key_id"`
	Test           bool    `json:"test"`
	Error          *string `json:"error"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	// BroadcastID (push) is the broadcast / scheduled send it was part of.
	BroadcastID *string `json:"broadcast_id,omitempty"`
	// ResentFrom is the original's id for a message resent from the panel
	// (the original's timeline has a "resent" event).
	ResentFrom *string        `json:"resent_from,omitempty"`
	Events     []MessageEvent `json:"events,omitempty"`
}

// ListMessagesInput filters Messages().List: Recipient, Reference and/or
// BroadcastID (the deliveries of one push broadcast).
type ListMessagesInput struct {
	Recipient   string
	Reference   string
	BroadcastID string
	Channel     string // email | sms | whatsapp | push | voice
	Limit       int    // 1–100, 0 = 20
	Before      string // the previous page's NextBefore
}

// MessageList is a page of Messages().List.
type MessageList struct {
	Messages   []MessageStatus `json:"messages"`
	NextBefore *string         `json:"next_before"`
}

// BuildListMessagesPayload is the body Messages().List posts.
func BuildListMessagesPayload(in ListMessagesInput) (map[string]any, error) {
	p := map[string]any{}
	if r := strings.TrimSpace(in.Recipient); r != "" {
		p["recipient"] = r
	}
	if in.Reference != "" {
		ref, err := ValidateReference(in.Reference)
		if err != nil {
			return nil, err
		}
		if ref != "" {
			p["reference"] = ref
		}
	}
	if in.BroadcastID != "" {
		if !uuidRe.MatchString(strings.TrimSpace(in.BroadcastID)) {
			return nil, validation("'broadcast_id' must be a broadcast id (UUID)")
		}
		p["broadcast_id"] = strings.TrimSpace(in.BroadcastID)
	}
	if len(p) == 0 {
		return nil, validation("Give Recipient, Reference or BroadcastID")
	}
	if in.Channel != "" {
		ch := strings.ToLower(in.Channel)
		switch ch {
		case "email", "sms", "whatsapp", "push", "voice":
			p["channel"] = ch
		default:
			return nil, validation("'channel' must be email, sms, whatsapp, push or voice")
		}
	}
	if in.Limit != 0 {
		if in.Limit < 1 || in.Limit > 100 {
			return nil, validation("'limit' must be from 1 to 100")
		}
		p["limit"] = in.Limit
	}
	if in.Before != "" {
		p["before"] = in.Before
	}
	return p, nil
}

// Messages is the message status API; get one with CatzConnect.Messages.
type Messages struct {
	c   *CatzConnect
	env *EnvValues
}

// Messages returns the message status API.
func (c *CatzConnect) Messages() *Messages { return &Messages{c: c} }

// WithEnv returns a copy that uses explicit credentials.
func (m *Messages) WithEnv(env *EnvValues) *Messages { return &Messages{c: m.c, env: env} }

// Get returns one message with its event timeline, by the message_id a send returned.
func (m *Messages) Get(ctx context.Context, messageID string) (*MessageStatus, error) {
	if !uuidRe.MatchString(strings.TrimSpace(messageID)) {
		return nil, validation("'message_id' must be a message id (UUID)")
	}
	d, err := m.c.sdkCall(ctx, "/sdk/messages/get", map[string]any{"message_id": strings.TrimSpace(messageID)}, m.env)
	if err != nil {
		return nil, err
	}
	var out MessageStatus
	if err := decodeInto(d, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns recent messages to a recipient and/or with a reference.
func (m *Messages) List(ctx context.Context, in ListMessagesInput) (*MessageList, error) {
	p, err := BuildListMessagesPayload(in)
	if err != nil {
		return nil, err
	}
	d, err := m.c.sdkCall(ctx, "/sdk/messages/list", p, m.env)
	if err != nil {
		return nil, err
	}
	var out MessageList
	if err := decodeInto(d, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── hosted OTP ──────────────────────────────────────────────────────────

// OtpStartInput is the input to Otp().Start. Zero numbers keep the server
// defaults (6 digits, 600 s, 5 attempts, 30 s cooldown).
type OtpStartInput struct {
	Channel               Channel // Email, SMS, WhatsApp or Voice
	To                    string
	Identity              string
	Template              string // "Otp" (default) or one of your templates
	Language              string
	CodeLength            int // 4–10
	TTLSeconds            int // 60–3600
	MaxAttempts           int // 1–10
	ResendCooldownSeconds int // 10–3600
	Gateway               string
	Sender                string
	DLTTemplateID         string
	Provider              string
	WhatsApp              *WhatsAppOptions
	Data                  map[string]string
	Reference             string
}

// OtpStart is the answer of Otp().Start. Code is set for test keys only.
type OtpStart struct {
	VerificationID    string  `json:"verification_id"`
	Channel           string  `json:"channel"`
	To                string  `json:"to"`
	Status            string  `json:"status"`
	ExpiresAt         string  `json:"expires_at"`
	ResendAvailableAt string  `json:"resend_available_at"`
	AttemptsRemaining int     `json:"attempts_remaining"`
	SendsRemaining    int     `json:"sends_remaining"`
	MessageID         *string `json:"message_id"`
	Test              bool    `json:"test"`
	Code              string  `json:"code,omitempty"`
}

// OtpVerifyInput: VerificationID, or Channel and To; and the Code.
type OtpVerifyInput struct {
	VerificationID string
	Channel        Channel
	To             string
	Code           string
}

// OtpVerified is the answer of a correct code.
type OtpVerified struct {
	Verified       bool   `json:"verified"`
	VerificationID string `json:"verification_id"`
	VerifiedAt     string `json:"verified_at"`
	Test           bool   `json:"test"`
}

func otpChannel(c Channel) (string, bool) {
	switch strings.ToLower(string(c)) {
	case "email":
		return "Email", true
	case "sms":
		return "SMS", true
	case "whatsapp":
		return "WhatsApp", true
	case "voice":
		return "Voice", true
	}
	return "", false
}

// BuildOtpStartPayload is the body Otp().Start posts.
func BuildOtpStartPayload(in OtpStartInput) (map[string]any, error) {
	ch, ok := otpChannel(in.Channel)
	if !ok {
		return nil, validation("'channel' must be Email, SMS, WhatsApp or Voice")
	}
	if strings.TrimSpace(in.To) == "" {
		return nil, validation("Missing 'to'")
	}
	p := map[string]any{"channel": ch, "to": strings.TrimSpace(in.To)}
	for k, v := range map[string]string{"identity": in.Identity, "template": in.Template, "language": in.Language,
		"gateway": in.Gateway, "sender": in.Sender, "dlt_template_id": in.DLTTemplateID, "provider": in.Provider} {
		if strings.TrimSpace(v) != "" {
			p[k] = strings.TrimSpace(v)
		}
	}
	nums := []struct {
		k        string
		v        int
		min, max int
	}{{"code_length", in.CodeLength, 4, 10}, {"ttl_seconds", in.TTLSeconds, 60, 3600}, {"max_attempts", in.MaxAttempts, 1, 10}, {"resend_cooldown_seconds", in.ResendCooldownSeconds, 10, 3600}}
	for _, n := range nums {
		if n.v == 0 {
			continue
		}
		if n.v < n.min || n.v > n.max {
			return nil, validation(fmt.Sprintf("'%s' must be from %d to %d", n.k, n.min, n.max))
		}
		p[n.k] = n.v
	}
	if in.WhatsApp != nil {
		p["whatsapp"] = in.WhatsApp
	}
	if len(in.Data) > 0 {
		p["data"] = in.Data
	}
	if in.Reference != "" {
		ref, err := ValidateReference(in.Reference)
		if err != nil {
			return nil, err
		}
		if ref != "" {
			p["reference"] = ref
		}
	}
	return p, nil
}

var otpCode = regexp.MustCompile(`^[A-Za-z0-9]{1,15}$`)

// BuildOtpVerifyPayload is the body Otp().Verify posts.
func BuildOtpVerifyPayload(in OtpVerifyInput) (map[string]any, error) {
	code := strings.NewReplacer(" ", "", "-", "").Replace(in.Code)
	if !otpCode.MatchString(code) {
		return nil, validation("'code' must be the 1–15 letters or digits that were sent")
	}
	if in.VerificationID != "" {
		if !uuidRe.MatchString(strings.TrimSpace(in.VerificationID)) {
			return nil, validation("'verification_id' must be a UUID")
		}
		return map[string]any{"verification_id": strings.TrimSpace(in.VerificationID), "code": code}, nil
	}
	ch, ok := otpChannel(in.Channel)
	if !ok || strings.TrimSpace(in.To) == "" {
		return nil, validation("Give VerificationID, or Channel and To")
	}
	return map[string]any{"channel": ch, "to": strings.TrimSpace(in.To), "code": code}, nil
}

// Otp is the hosted one-time code API; get one with CatzConnect.Otp.
// Refusals are *Error with Code otp_invalid (Details["attempts_remaining"]),
// otp_expired, otp_attempts_exceeded, otp_resend_cooldown
// (Details["retry_after"]), otp_send_limit, otp_not_found, otp_already_verified.
type Otp struct {
	c   *CatzConnect
	env *EnvValues
}

// Otp returns the hosted OTP API.
func (c *CatzConnect) Otp() *Otp { return &Otp{c: c} }

// WithEnv returns a copy that uses explicit credentials.
func (o *Otp) WithEnv(env *EnvValues) *Otp { return &Otp{c: o.c, env: env} }

// Start sends a code (again: a resend replaces it, after the cooldown).
func (o *Otp) Start(ctx context.Context, in OtpStartInput) (*OtpStart, error) {
	p, err := BuildOtpStartPayload(in)
	if err != nil {
		return nil, err
	}
	d, err := o.c.sdkCall(ctx, "/sdk/otp/start", p, o.env)
	if err != nil {
		return nil, err
	}
	var out OtpStart
	return &out, decodeInto(d, &out)
}

// Verify checks a code; it returns an error unless the code is correct.
func (o *Otp) Verify(ctx context.Context, in OtpVerifyInput) (*OtpVerified, error) {
	p, err := BuildOtpVerifyPayload(in)
	if err != nil {
		return nil, err
	}
	d, err := o.c.sdkCall(ctx, "/sdk/otp/verify", p, o.env)
	if err != nil {
		return nil, err
	}
	var out OtpVerified
	return &out, decodeInto(d, &out)
}

// ─── management API ──────────────────────────────────────────────────────

// Manage is the management API (templates, webhooks, suppressions, API
// keys). It needs a live server key with the management scope on a plan
// with the management API. Answers are the server's `data` objects.
type Manage struct {
	c   *CatzConnect
	env *EnvValues
}

// Manage returns the management API.
func (c *CatzConnect) Manage() *Manage { return &Manage{c: c} }

// WithEnv returns a copy that uses explicit credentials.
func (m *Manage) WithEnv(env *EnvValues) *Manage { return &Manage{c: m.c, env: env} }

var manageOps = map[string]bool{
	"templates/list": true, "templates/get": true, "templates/save": true, "templates/delete": true,
	"templates/variant_save": true, "templates/variant_delete": true,
	"webhooks/list": true, "webhooks/save": true, "webhooks/delete": true,
	"suppressions/list": true, "suppressions/add": true, "suppressions/remove": true,
	"keys/list": true, "keys/create": true, "keys/rotate": true, "keys/revoke": true, "keys/policy": true,
}

// Call runs one operation, e.g. Call(ctx, "templates", "save", map[string]any{...}).
// The fields of each operation are in the API reference.
func (m *Manage) Call(ctx context.Context, resource, op string, body map[string]any) (map[string]any, error) {
	if !manageOps[resource+"/"+op] {
		return nil, validation("Unknown management operation " + resource + "/" + op)
	}
	if body == nil {
		body = map[string]any{}
	}
	return m.c.sdkCall(ctx, "/sdk/manage/"+resource+"/"+op, body, m.env)
}

// ListTemplates lists templates (channel "" = all, else Email, SMS or Voice).
func (m *Manage) ListTemplates(ctx context.Context, channel string) (map[string]any, error) {
	b := map[string]any{}
	if channel != "" {
		b["channel"] = channel
	}
	return m.Call(ctx, "templates", "list", b)
}

// SaveTemplate creates (no "id") or updates a template: name, channel,
// subject, body, dlt_template_id.
func (m *Manage) SaveTemplate(ctx context.Context, t map[string]any) (map[string]any, error) {
	if s, _ := t["name"].(string); strings.TrimSpace(s) == "" {
		return nil, validation("Missing 'name'")
	}
	return m.Call(ctx, "templates", "save", t)
}

// SaveTemplateVariant saves a language version (template_id, language, subject?, body).
func (m *Manage) SaveTemplateVariant(ctx context.Context, v map[string]any) (map[string]any, error) {
	return m.Call(ctx, "templates", "variant_save", v)
}

// SaveWebhook creates (answer has the signing secret, once) or updates a webhook.
func (m *Manage) SaveWebhook(ctx context.Context, w map[string]any) (map[string]any, error) {
	if ev, _ := w["events"].([]string); len(ev) == 0 {
		if evAny, _ := w["events"].([]any); len(evAny) == 0 {
			return nil, validation("Choose at least one event")
		}
	}
	return m.Call(ctx, "webhooks", "save", w)
}

// AddSuppression adds an address to a channel's list: "email" (address),
// "sms" / "whatsapp" / "voice" (E.164 number) or "push" (external_user_id).
func (m *Manage) AddSuppression(ctx context.Context, channel, address string) (map[string]any, error) {
	return m.AddSuppressionWith(ctx, SuppressionInput{Channel: channel, Address: address})
}

// RemoveSuppression removes an address from a channel (channel "" works
// for email addresses only), every topic.
func (m *Manage) RemoveSuppression(ctx context.Context, channel, address string) (map[string]any, error) {
	return m.RemoveSuppressionWith(ctx, SuppressionInput{Channel: channel, Address: address})
}

// SuppressionInput is one suppression entry: Channel + Address (+ Topic,
// email only), or ID (from a list) for RemoveSuppressionWith.
type SuppressionInput struct {
	ID      string
	Channel string
	Address string
	Topic   string
	Detail  string
}

func suppressionBody(in SuppressionInput) (map[string]any, error) {
	b := map[string]any{}
	if strings.TrimSpace(in.Address) == "" {
		return nil, validation("Missing 'address'")
	}
	b["address"] = strings.TrimSpace(in.Address)
	if in.Channel != "" {
		b["channel"] = in.Channel
	}
	if in.Topic != "" {
		if !strings.EqualFold(in.Channel, "email") && !(in.Channel == "" && strings.Contains(in.Address, "@")) {
			return nil, validation("'topic' applies to email suppressions only")
		}
		if !EmailTopicKey.MatchString(in.Topic) {
			return nil, validation("'topic' must be a topic key")
		}
		b["topic"] = in.Topic
	}
	return b, nil
}

// AddSuppressionWith adds an entry with an optional Topic (email) and Detail.
func (m *Manage) AddSuppressionWith(ctx context.Context, in SuppressionInput) (map[string]any, error) {
	if in.Channel == "" {
		return nil, validation("Missing 'channel'")
	}
	b, err := suppressionBody(in)
	if err != nil {
		return nil, err
	}
	if in.Detail != "" {
		b["detail"] = in.Detail
	}
	return m.Call(ctx, "suppressions", "add", b)
}

// RemoveSuppressionWith removes by ID, or Channel + Address (+ Topic: only
// that topic's entry).
func (m *Manage) RemoveSuppressionWith(ctx context.Context, in SuppressionInput) (map[string]any, error) {
	if in.ID != "" {
		if !uuidRe.MatchString(in.ID) {
			return nil, validation("'id' must be a suppression id (UUID)")
		}
		return m.Call(ctx, "suppressions", "remove", map[string]any{"id": in.ID})
	}
	b, err := suppressionBody(in)
	if err != nil {
		return nil, err
	}
	return m.Call(ctx, "suppressions", "remove", b)
}

// CreatedKey holds the secrets of a created or rotated key: shown only once.
type CreatedKey struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind,omitempty"`
	APIKey          string `json:"api_key"`
	PrivateKey      string `json:"private_key,omitempty"`
	ServerPublicKey string `json:"server_public_key,omitempty"`
	APIKeyShort     string `json:"api_key_short"`
}

// CreateKey creates a key: name, kind ("server" or "test"), channels
// (names) or channel_ids, identity ids, policy. The answer has its secrets.
func (m *Manage) CreateKey(ctx context.Context, k map[string]any) (*CreatedKey, error) {
	if kind, ok := k["kind"].(string); ok && kind != "server" && kind != "test" {
		return nil, validation("'kind' must be server or test")
	}
	d, err := m.Call(ctx, "keys", "create", k)
	if err != nil {
		return nil, err
	}
	var out CreatedKey
	return &out, decodeInto(d, &out)
}

// RotateKey issues a new secret (and with rotateEte a new key pair).
func (m *Manage) RotateKey(ctx context.Context, id string, rotateEte bool) (*CreatedKey, error) {
	d, err := m.Call(ctx, "keys", "rotate", map[string]any{"id": id, "rotate_ete": rotateEte})
	if err != nil {
		return nil, err
	}
	var out CreatedKey
	return &out, decodeInto(d, &out)
}

// RevokeKey revokes a key at once.
func (m *Manage) RevokeKey(ctx context.Context, id string) (map[string]any, error) {
	return m.Call(ctx, "keys", "revoke", map[string]any{"id": id})
}

// ─── push device registry ────────────────────────────────────────────────

// RegisterDeviceInput registers an app device with POST /push/register.
type RegisterDeviceInput struct {
	PublishableKey string
	Token          string
	Platform       string // ios | android | web
	ExternalUserID string
	UserHash       string // ComputeUserHash on your server
	AppVersion     string
	DeviceKey      string
}

// BuildRegisterDevicePayload is the body PushDevices().Register posts.
func BuildRegisterDevicePayload(in RegisterDeviceInput) (map[string]any, error) {
	if l := len(strings.TrimSpace(in.PublishableKey)); l < 10 || l > 100 {
		return nil, validation("'publishable_key' is required")
	}
	if l := len(strings.TrimSpace(in.Token)); l < 32 || l > 4096 {
		return nil, validation("'token' must be the device's FCM registration token")
	}
	switch in.Platform {
	case "ios", "android", "web":
	default:
		return nil, validation("'platform' must be ios, android or web")
	}
	p := map[string]any{"publishable_key": strings.TrimSpace(in.PublishableKey), "token": strings.TrimSpace(in.Token), "platform": in.Platform}
	if in.ExternalUserID != "" {
		if utf8.RuneCountInString(in.ExternalUserID) > 128 || strings.IndexFunc(in.ExternalUserID, unicode.IsControl) >= 0 {
			return nil, validation("'external_user_id' must be 1–128 characters")
		}
		p["external_user_id"] = in.ExternalUserID
	}
	if in.UserHash != "" {
		p["user_hash"] = in.UserHash
	}
	if in.AppVersion != "" {
		if utf8.RuneCountInString(in.AppVersion) > 40 {
			return nil, validation("'app_version' is at most 40 characters")
		}
		p["app_version"] = in.AppVersion
	}
	if in.DeviceKey != "" {
		p["device_key"] = in.DeviceKey
	}
	return p, nil
}

// PushDevices wraps the public device endpoints (publishable key, no API key).
type PushDevices struct {
	c       *CatzConnect
	baseURL string
}

// PushDevices returns the device registry API; baseURL "" uses
// CATZCONNECT_BASE_URL or https://api.catzconnect.com.
func (c *CatzConnect) PushDevices(baseURL string) *PushDevices {
	return &PushDevices{c: c, baseURL: baseURL}
}

func dataOf(res map[string]any) map[string]any {
	if d, ok := res["data"].(map[string]any); ok {
		return d
	}
	return res
}

// Register registers or refreshes a device: {device_id, created}.
func (p *PushDevices) Register(ctx context.Context, in RegisterDeviceInput) (map[string]any, error) {
	body, err := BuildRegisterDevicePayload(in)
	if err != nil {
		return nil, err
	}
	res, err := p.c.http.postPublic(ctx, p.baseURL, "/push/register", body)
	if err != nil {
		return nil, err
	}
	return dataOf(res), nil
}

// Unregister forgets a token: {removed}.
func (p *PushDevices) Unregister(ctx context.Context, publishableKey, token string) (map[string]any, error) {
	res, err := p.c.http.postPublic(ctx, p.baseURL, "/push/unregister", map[string]any{"publishable_key": publishableKey, "token": token})
	if err != nil {
		return nil, err
	}
	return dataOf(res), nil
}

// SubscribeTopics subscribes this registered device to 1–20 push topics:
// {topics}. 404 when the token was not registered first.
func (p *PushDevices) SubscribeTopics(ctx context.Context, publishableKey, token string, topics []string) (map[string]any, error) {
	return p.topicsCall(ctx, "/push/topics/subscribe", publishableKey, token, topics)
}

// UnsubscribeTopics unsubscribes this registered device from push topics.
func (p *PushDevices) UnsubscribeTopics(ctx context.Context, publishableKey, token string, topics []string) (map[string]any, error) {
	return p.topicsCall(ctx, "/push/topics/unsubscribe", publishableKey, token, topics)
}

func (p *PushDevices) topicsCall(ctx context.Context, path, publishableKey, token string, topics []string) (map[string]any, error) {
	if err := checkPushTopics(topics, 20); err != nil {
		return nil, err
	}
	res, err := p.c.http.postPublic(ctx, p.baseURL, path, map[string]any{"publishable_key": publishableKey, "token": token, "topics": topics})
	if err != nil {
		return nil, err
	}
	return dataOf(res), nil
}

func checkPushTopics(topics []string, max int) error {
	if len(topics) == 0 || len(topics) > max {
		return validation(fmt.Sprintf("'topics' must list 1–%d topic names", max))
	}
	for _, t := range topics {
		if !PushTopicName.MatchString(t) {
			return validation("A push topic is 1–64 characters of letters, digits, _ . ~ -")
		}
	}
	return nil
}

// Opened reports a notification opened (messageID = its catz_id) without the
// device's token: the open is recorded but not verified, so no push.opened
// webhook fires. Prefer OpenedWithToken whenever the app has its FCM token.
// {recorded, verified}.
func (p *PushDevices) Opened(ctx context.Context, publishableKey, messageID string) (map[string]any, error) {
	return p.OpenedWithToken(ctx, publishableKey, messageID, "")
}

// OpenedWithToken reports a notification opened with the FCM registration
// token of the device that opened it: the open is verified (only the device
// the notification was delivered to can report it) and the first verified
// report fires the push.opened webhook. An empty token behaves like Opened.
// {recorded, verified}.
func (p *PushDevices) OpenedWithToken(ctx context.Context, publishableKey, messageID, token string) (map[string]any, error) {
	if !uuidRe.MatchString(messageID) {
		return nil, validation("'message_id' must be the catz_id from the notification")
	}
	body := map[string]any{"publishable_key": publishableKey, "message_id": messageID}
	if t := strings.TrimSpace(token); t != "" {
		if len(t) < 32 || len(t) > 4096 {
			return nil, validation("'token' must be the device's FCM registration token")
		}
		body["token"] = t
	}
	res, err := p.c.http.postPublic(ctx, p.baseURL, "/push/opened", body)
	if err != nil {
		return nil, err
	}
	return dataOf(res), nil
}

// ─── push broadcasts and topics (/sdk/push/*) ────────────────────────────

// PushBroadcast is a broadcast / scheduled push (Push().Broadcast).
// Status: scheduled → dispatching → completed | canceled | failed.
type PushBroadcast struct {
	ID              string  `json:"id"`
	PushProjectID   string  `json:"push_project_id"`
	ProjectLabel    string  `json:"project_label"`
	FirebaseProject string  `json:"firebase_project"`
	KeyID           *string `json:"key_id"`
	Audience        string  `json:"audience"`
	Topic           *string `json:"topic"`
	ExternalUserID  *string `json:"external_user_id"`
	TitleHint       *string `json:"title_hint"`
	SendAt          string  `json:"send_at"`
	Status          string  `json:"status"`
	Queued          int     `json:"queued"`
	Skipped         int     `json:"skipped"`
	OverQuota       int     `json:"over_quota"`
	Error           *string `json:"error"`
	ErrorCode       *string `json:"error_code"`
	StartedAt       *string `json:"started_at"`
	CompletedAt     *string `json:"completed_at"`
	CanceledAt      *string `json:"canceled_at"`
	CreatedAt       string  `json:"created_at"`
}

// PushTopicChange subscribes / unsubscribes registered devices: Tokens
// (≤ 1000 FCM tokens) and/or every device of ExternalUserID.
type PushTopicChange struct {
	Identity       string // the Firebase project id
	Topic          string
	Tokens         []string
	ExternalUserID string
}

// BuildPushTopicChangePayload is the body Push().Subscribe / Unsubscribe posts.
func BuildPushTopicChangePayload(in PushTopicChange) (map[string]any, error) {
	if strings.TrimSpace(in.Identity) == "" {
		return nil, validation("Missing 'identity'")
	}
	if !PushTopicName.MatchString(in.Topic) {
		return nil, validation("A push topic is 1–64 characters of letters, digits, _ . ~ -")
	}
	if len(in.Tokens) > 1000 {
		return nil, validation("'tokens' must list at most 1000 tokens")
	}
	if len(in.Tokens) == 0 && strings.TrimSpace(in.ExternalUserID) == "" {
		return nil, validation("Give Tokens (registered devices) or ExternalUserID")
	}
	p := map[string]any{"identity": strings.TrimSpace(in.Identity), "topic": in.Topic}
	if len(in.Tokens) > 0 {
		p["tokens"] = in.Tokens
	}
	if u := strings.TrimSpace(in.ExternalUserID); u != "" {
		p["external_user_id"] = u
	}
	return p, nil
}

// Push manages broadcasts / scheduled pushes (created by Send with
// Payload.Push) and push topic subscriptions from your server. Server keys
// with the Push channel; app and test keys are refused.
type Push struct {
	c   *CatzConnect
	env *EnvValues
}

// Push returns the push broadcast and topic API.
func (c *CatzConnect) Push() *Push { return &Push{c: c} }

// WithEnv returns a copy that uses explicit credentials.
func (p *Push) WithEnv(env *EnvValues) *Push { return &Push{c: p.c, env: env} }

// Broadcast returns a broadcast's status and counters.
func (p *Push) Broadcast(ctx context.Context, broadcastID string) (*PushBroadcast, error) {
	if !uuidRe.MatchString(broadcastID) {
		return nil, validation("'broadcast_id' must be a broadcast id (UUID)")
	}
	d, err := p.c.sdkCall(ctx, "/sdk/push/broadcast", map[string]any{"broadcast_id": broadcastID}, p.env)
	if err != nil {
		return nil, err
	}
	var out PushBroadcast
	return &out, decodeInto(d, &out)
}

// Cancel stops a broadcast that has not finished (CodeNotCancelable once it has).
func (p *Push) Cancel(ctx context.Context, broadcastID string) (map[string]any, error) {
	if !uuidRe.MatchString(broadcastID) {
		return nil, validation("'broadcast_id' must be a broadcast id (UUID)")
	}
	return p.c.sdkCall(ctx, "/sdk/push/cancel", map[string]any{"broadcast_id": broadcastID}, p.env)
}

// Subscribe subscribes registered devices to a topic: {topic, devices}.
func (p *Push) Subscribe(ctx context.Context, in PushTopicChange) (map[string]any, error) {
	b, err := BuildPushTopicChangePayload(in)
	if err != nil {
		return nil, err
	}
	return p.c.sdkCall(ctx, "/sdk/push/topic/subscribe", b, p.env)
}

// Unsubscribe unsubscribes registered devices from a topic: {topic, devices}.
func (p *Push) Unsubscribe(ctx context.Context, in PushTopicChange) (map[string]any, error) {
	b, err := BuildPushTopicChangePayload(in)
	if err != nil {
		return nil, err
	}
	return p.c.sdkCall(ctx, "/sdk/push/topic/unsubscribe", b, p.env)
}

// Topics lists a project's topics: {topics: [{topic, devices, last_subscribed_at}]}.
func (p *Push) Topics(ctx context.Context, identity string) (map[string]any, error) {
	if strings.TrimSpace(identity) == "" {
		return nil, validation("Missing 'identity'")
	}
	return p.c.sdkCall(ctx, "/sdk/push/topics", map[string]any{"identity": strings.TrimSpace(identity)}, p.env)
}
