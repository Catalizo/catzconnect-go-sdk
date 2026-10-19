package catzconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Voice & Video calls from your backend: create a call, mint a user token
// for the browser/app client, read a call, end it. Every request uses the
// same API key, encryption and headers as Send.
//
//	call, err := catzconnect.Default.Calls().Create(ctx, catzconnect.CreateCallInput{
//	    Mode: catzconnect.CallModeVideo, From: "alice", To: []string{"bob"},
//	})
//	tok, err := catzconnect.Default.Calls().Token(ctx, "alice", 0)

// CallMode is "audio" or "video".
type CallMode string

const (
	CallModeAudio CallMode = "audio"
	CallModeVideo CallMode = "video"
)

// Limits the server applies, checked here first.
const (
	CallMaxCallees       = 49
	CallUserIDMax        = 128
	CallMetadataMaxBytes = 4096
	CallTokenTTLMin      = 60
	CallTokenTTLMax      = 86400
)

// CreateCallInput is the input to Calls.Create.
type CreateCallInput struct {
	Mode CallMode
	// From is the caller's external user id (your user id, ≤ 128 characters).
	From string
	// To lists the callees' user ids: 1–49, unique, not including From.
	To []string
	// Engine is "auto" (empty: the org's setting), "catz", or a configured
	// provider's id or label in the key's scope.
	Engine string
	// Ring the callees (WebSocket + push); nil means the server default (true).
	Ring *bool
	// Metadata is your own JSON object, ≤ 4 KB, delivered with "incoming".
	Metadata map[string]any
	// MaxParticipants caps the call (caller included), 2–50; 0 = default.
	MaxParticipants int
}

// Bool returns a pointer to b, for CreateCallInput.Ring.
func Bool(b bool) *bool { return &b }

// CallParticipant is one participant as the server reports it.
type CallParticipant struct {
	UserID     string  `json:"user_id"`
	Role       string  `json:"role"`
	State      string  `json:"state"`
	JoinedAt   *string `json:"joined_at,omitempty"`
	LeftAt     *string `json:"left_at,omitempty"`
	Seconds    int     `json:"seconds,omitempty"`
	Connection *string `json:"connection,omitempty"`
}

// CallProvider is the configured provider a call runs on (engine not
// "catz"): its row id, the label set in the panel and the registry name
// (agora, livekit, …).
type CallProvider struct {
	ID    string  `json:"id"`
	Label *string `json:"label,omitempty"`
	Name  string  `json:"name"`
}

// Call is a call as Calls.Create and Calls.Get return it.
type Call struct {
	ID     string `json:"id"`
	Engine string `json:"engine"`
	// Provider is nil for engine "catz".
	Provider      *CallProvider     `json:"provider,omitempty"`
	Mode          CallMode          `json:"mode"`
	Status        string            `json:"status"`
	Room          string            `json:"room"`
	Participants  []CallParticipant `json:"participants"`
	CreatedAt     string            `json:"created_at"`
	StartedAt     *string           `json:"started_at,omitempty"`
	EndedAt       *string           `json:"ended_at,omitempty"`
	DurationSecs  *int              `json:"duration_secs,omitempty"`
	BilledMinutes *int              `json:"billed_minutes,omitempty"`
	EndReason     *string           `json:"end_reason,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// Timestamp holds a time the server sent as a string or a number, as text.
type Timestamp string

// UnmarshalJSON accepts "2026-10-18T…" and 1760000000 alike.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*t = Timestamp(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*t = Timestamp(n.String())
	return nil
}

// CallToken is a user token for the browser/app client. It never contains
// the API key.
type CallToken struct {
	Token     string    `json:"token"`
	ExpiresAt Timestamp `json:"expires_at"`
	// WSURL is the signaling URL for the client (CatzCallClient({ url })).
	WSURL string `json:"ws_url"`
}

// EndCallResult is the server's answer to Calls.End.
type EndCallResult struct {
	Success bool
	Message string
}

// Calls is the calls API; get one with CatzConnect.Calls.
type Calls struct {
	c   *CatzConnect
	env *EnvValues
}

// Calls returns the calls API, reading credentials from the environment.
func (c *CatzConnect) Calls() *Calls { return &Calls{c: c} }

// WithEnv returns a copy that uses explicit credentials.
func (s *Calls) WithEnv(env *EnvValues) *Calls { return &Calls{c: s.c, env: env} }

func validateCallUserID(id, field string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("Missing %s", field)
	}
	if utf8.RuneCountInString(id) > CallUserIDMax {
		return fmt.Errorf("%s must be at most %d characters", field, CallUserIDMax)
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains control characters", field)
		}
	}
	return nil
}

func validateCallID(id string) (string, error) {
	t := strings.TrimSpace(id)
	if t == "" {
		return "", fmt.Errorf("Missing call id")
	}
	if len(t) > 128 || strings.IndexFunc(t, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("Invalid call id")
	}
	return t, nil
}

// BuildCreateCallPayload is the JSON body (before encryption) Calls.Create
// posts, after the server's checks.
func BuildCreateCallPayload(in CreateCallInput) (map[string]any, error) {
	if in.Mode != CallModeAudio && in.Mode != CallModeVideo {
		return nil, fmt.Errorf("'mode' must be \"audio\" or \"video\"")
	}
	if err := validateCallUserID(in.From, "'from'"); err != nil {
		return nil, err
	}
	if len(in.To) == 0 {
		return nil, fmt.Errorf("'to' must list at least one user id")
	}
	if len(in.To) > CallMaxCallees {
		return nil, fmt.Errorf("'to' can list at most %d user ids", CallMaxCallees)
	}
	seen := make(map[string]bool, len(in.To))
	for _, u := range in.To {
		if err := validateCallUserID(u, "'to' entry"); err != nil {
			return nil, err
		}
		if u == in.From {
			return nil, fmt.Errorf("'to' must not include the caller ('from')")
		}
		if seen[u] {
			return nil, fmt.Errorf("'to' lists '%s' twice", u)
		}
		seen[u] = true
	}

	p := map[string]any{"mode": string(in.Mode), "from": in.From, "to": append([]string{}, in.To...)}
	if e := strings.TrimSpace(in.Engine); e != "" {
		p["engine"] = e
	}
	if in.Ring != nil {
		p["ring"] = *in.Ring
	}
	if in.Metadata != nil {
		b, err := marshalNoHTMLEscape(in.Metadata)
		if err != nil {
			return nil, fmt.Errorf("'metadata' must be serialisable as JSON")
		}
		if len(b) > CallMetadataMaxBytes {
			return nil, fmt.Errorf("'metadata' must be at most 4 KB as JSON")
		}
		p["metadata"] = in.Metadata
	}
	if in.MaxParticipants != 0 {
		n := in.MaxParticipants
		if n < 2 || n > CallMaxCallees+1 {
			return nil, fmt.Errorf("'max_participants' must be from 2 to %d", CallMaxCallees+1)
		}
		if len(in.To)+1 > n {
			return nil, fmt.Errorf("%d participants do not fit 'max_participants' %d", len(in.To)+1, n)
		}
		p["max_participants"] = n
	}
	return p, nil
}

// BuildCallTokenPayload is the JSON body Calls.Token posts. ttlSeconds 0
// leaves the server default (3600).
func BuildCallTokenPayload(userID string, ttlSeconds int) (map[string]any, error) {
	if err := validateCallUserID(userID, "'user_id'"); err != nil {
		return nil, err
	}
	p := map[string]any{"user_id": userID}
	if ttlSeconds != 0 {
		if ttlSeconds < CallTokenTTLMin || ttlSeconds > CallTokenTTLMax {
			return nil, fmt.Errorf("'ttl_seconds' must be from %d to %d", CallTokenTTLMin, CallTokenTTLMax)
		}
		p["ttl_seconds"] = ttlSeconds
	}
	return p, nil
}

func (s *Calls) post(ctx context.Context, path string, payload map[string]any) (map[string]any, error) {
	enc, err := encrypt(payload, s.env)
	if err != nil {
		return nil, err
	}
	res, err := s.c.http.postCtx(ctx, path, enc, s.env)
	if err != nil {
		return nil, err
	}
	res, err = openResponse(res, s.env)
	if err != nil {
		return nil, err
	}
	if ok, isBool := res["success"].(bool); isBool && !ok {
		b, _ := json.Marshal(res)
		return nil, apiError(200, b)
	}
	return res, nil
}

// field reads name from the top level or from data.
func field(res map[string]any, name string) (any, bool) {
	if v, ok := res[name]; ok {
		return v, true
	}
	if d, ok := res["data"].(map[string]any); ok {
		v, ok := d[name]
		return v, ok
	}
	return nil, false
}

func decodeField(res map[string]any, name string, out any) error {
	v, ok := field(res, name)
	if !ok {
		return &Error{Kind: KindJSON, Message: "response has no " + name}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}
	if err := json.Unmarshal(b, out); err != nil {
		return &Error{Kind: KindJSON, Message: err.Error(), Err: err}
	}
	return nil
}

func decodeCall(res map[string]any) (*Call, error) {
	var call Call
	if err := decodeField(res, "call", &call); err != nil {
		return nil, err
	}
	if call.ID == "" {
		return nil, &Error{Kind: KindJSON, Message: "response has no call.id"}
	}
	return &call, nil
}

// Create creates a call and (unless Ring is false) rings the callees.
func (s *Calls) Create(ctx context.Context, in CreateCallInput) (*Call, error) {
	p, err := BuildCreateCallPayload(in)
	if err != nil {
		return nil, &Error{Kind: KindValidation, Message: err.Error(), Err: err}
	}
	res, err := s.post(ctx, "/sdk/calls/create", p)
	if err != nil {
		return nil, err
	}
	return decodeCall(res)
}

// Token mints a user token for userID; ttlSeconds 60–86400, or 0 for the
// server default (3600).
func (s *Calls) Token(ctx context.Context, userID string, ttlSeconds int) (*CallToken, error) {
	p, err := BuildCallTokenPayload(userID, ttlSeconds)
	if err != nil {
		return nil, &Error{Kind: KindValidation, Message: err.Error(), Err: err}
	}
	res, err := s.post(ctx, "/sdk/calls/token", p)
	if err != nil {
		return nil, err
	}
	var t CallToken
	if v, ok := field(res, "token"); ok {
		t.Token, _ = v.(string)
	}
	if t.Token == "" {
		return nil, &Error{Kind: KindJSON, Message: "response has no token"}
	}
	_ = decodeField(res, "expires_at", &t.ExpiresAt)
	if v, ok := field(res, "ws_url"); ok {
		t.WSURL, _ = v.(string)
	}
	return &t, nil
}

// Get reads a call with its participants, duration, billed minutes and end reason.
func (s *Calls) Get(ctx context.Context, callID string) (*Call, error) {
	id, err := validateCallID(callID)
	if err != nil {
		return nil, &Error{Kind: KindValidation, Message: err.Error(), Err: err}
	}
	res, err := s.post(ctx, "/sdk/calls/get", map[string]any{"call_id": id})
	if err != nil {
		return nil, err
	}
	return decodeCall(res)
}

// End ends a call for everyone; reason is optional.
func (s *Calls) End(ctx context.Context, callID, reason string) (*EndCallResult, error) {
	id, err := validateCallID(callID)
	if err != nil {
		return nil, &Error{Kind: KindValidation, Message: err.Error(), Err: err}
	}
	p := map[string]any{"call_id": id}
	if r := strings.TrimSpace(reason); r != "" {
		p["reason"] = r
	}
	res, err := s.post(ctx, "/sdk/calls/end", p)
	if err != nil {
		return nil, err
	}
	out := &EndCallResult{Success: true}
	out.Message, _ = res["message"].(string)
	return out, nil
}
