package catzconnect

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// ComputeUserHash returns the user_hash for POST /push/register: the hex
// HMAC-SHA256 of externalUserID keyed with the push project's identity
// secret (pis_…, Push → Projects in the panel).
//
// Compute it on your server and hand it to your app along with the user id.
// The identity secret must never ship inside the app — anyone holding it
// could register their device as any of your users. Without a valid hash the
// device is registered with no user.
func ComputeUserHash(externalUserID, identitySecret string) string {
	mac := hmac.New(sha256.New, []byte(identitySecret))
	mac.Write([]byte(externalUserID))
	return hex.EncodeToString(mac.Sum(nil))
}
