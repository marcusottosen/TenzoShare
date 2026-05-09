package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

const unsubscribeSuffix = ":unsubscribe_v1"

// UnsubscribeToken returns a URL-safe token that encodes the recipient email
// and an HMAC-SHA256 signature keyed on pepper. The token can be embedded in
// an unsubscribe link and later verified with VerifyUnsubscribeToken.
//
// Format: base64url(email) "." base64url(HMAC-SHA256(email, pepper+suffix))
func UnsubscribeToken(email, pepper string) string {
	emailEnc := base64.RawURLEncoding.EncodeToString([]byte(email))
	sig := computeUnsubHMAC(email, pepper)
	return emailEnc + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// VerifyUnsubscribeToken validates token and returns the encoded email if the
// HMAC signature is correct. Returns ("", false) on any validation failure.
func VerifyUnsubscribeToken(token, pepper string) (string, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	emailBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(emailBytes) == 0 {
		return "", false
	}
	email := string(emailBytes)

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	expected := computeUnsubHMAC(email, pepper)
	if !hmac.Equal(sigBytes, expected) {
		return "", false
	}
	return email, true
}

func computeUnsubHMAC(email, pepper string) []byte {
	mac := hmac.New(sha256.New, []byte(pepper+unsubscribeSuffix))
	mac.Write([]byte(email))
	return mac.Sum(nil)
}
