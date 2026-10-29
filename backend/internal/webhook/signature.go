package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ComputeHMACSHA256 computes the lowercase hexadecimal HMAC-SHA256 signature over raw bytes.
func ComputeHMACSHA256(secret string, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMACSHA256 verifies a candidate hex signature against the expected HMAC-SHA256 over rawBody
// using constant-time comparison (hmac.Equal) to prevent timing attacks.
func VerifyHMACSHA256(secret string, rawBody []byte, candidateSignature string) bool {
	if secret == "" || candidateSignature == "" {
		return false
	}

	cleanCandidate := strings.TrimSpace(strings.ToLower(candidateSignature))
	cleanCandidate = strings.TrimPrefix(cleanCandidate, "sha256=")

	candidateBytes, err := hex.DecodeString(cleanCandidate)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expectedBytes := mac.Sum(nil)

	return hmac.Equal(expectedBytes, candidateBytes)
}
