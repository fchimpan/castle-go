package castle

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// ErrWebhookVerification indicates a missing or invalid webhook signature or secret.
var ErrWebhookVerification = errors.New("castle: webhook signature verification failed")

// VerifyWebhookSignature verifies the X-Castle-Signature header against the exact
// request body bytes using the API secret selected for webhook signing in Castle.
// Call it before parsing or processing the body. It returns ErrWebhookVerification
// for an empty secret, missing or malformed signature, or signature mismatch.
func VerifyWebhookSignature(body []byte, signature, apiSecret string) error {
	if apiSecret == "" || len(signature) != base64.StdEncoding.EncodedLen(sha256.Size) {
		return ErrWebhookVerification
	}
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return ErrWebhookVerification
	}
	return nil
}
