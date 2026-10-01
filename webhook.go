package catzconnect

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// DefaultWebhookTolerance is how far a webhook's timestamp may be from now
// before VerifyWebhookSignature rejects it as a possible replay.
const DefaultWebhookTolerance = 5 * time.Minute

// VerifyWebhookSignature checks a webhook delivery's Catz-Signature header.
//
// The header is "t=<unix seconds>,v1=<hex HMAC-SHA256>", where the MAC is
// taken over "<t>.<raw body>" with the webhook's signing secret (whsec_…) as
// the key. Pass the body exactly as received, before any JSON decoding.
// Timestamps more than DefaultWebhookTolerance from now are rejected.
func VerifyWebhookSignature(rawBody []byte, header, secret string) bool {
	return VerifyWebhookSignatureAt(rawBody, header, secret, time.Now(), DefaultWebhookTolerance)
}

// VerifyWebhookSignatureAt is VerifyWebhookSignature with an explicit clock
// and tolerance. A tolerance of zero or less skips the timestamp check.
func VerifyWebhookSignatureAt(rawBody []byte, header, secret string, now time.Time, tolerance time.Duration) bool {
	if header == "" || secret == "" {
		return false
	}

	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "t":
			ts = strings.TrimSpace(v)
		case "v1":
			sigs = append(sigs, strings.TrimSpace(v))
		}
	}
	if ts == "" || len(sigs) == 0 {
		return false
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || t < 0 {
		return false
	}
	if tolerance > 0 {
		diff := now.Sub(time.Unix(t, 0))
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			return false
		}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	expected := mac.Sum(nil)

	for _, s := range sigs {
		got, err := hex.DecodeString(s)
		if err != nil || len(got) != sha256.Size {
			continue
		}
		if hmac.Equal(expected, got) {
			return true
		}
	}
	return false
}
