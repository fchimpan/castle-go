package castle_test

import (
	"errors"
	"strings"
	"testing"

	castle "github.com/fchimpan/castle-go"
)

func TestVerifyWebhookSignature(t *testing.T) {
	t.Parallel()
	// Fixed vectors computed independently with Python hmac and base64, following
	// https://docs.castle.io/docs/webhooks (raw bytes, HMAC-SHA256, standard Base64).
	const body = `{"type":"$login"}`
	const secret = "webhook-test-secret"
	const signature = "ft28OCuRuuPXUwhqVp0fvxX/ePW2UiKjBXYDS9HVSzA="
	for _, tc := range []struct {
		name      string
		body      string
		signature string
		secret    string
		wantError bool
	}{
		{
			name:      "valid",
			body:      body,
			signature: signature,
			secret:    secret,
			wantError: false,
		},
		{
			name:      "changed body",
			body:      body + " ",
			signature: signature,
			secret:    secret,
			wantError: true,
		},
		{
			name:      "wrong secret",
			body:      body,
			signature: signature,
			secret:    "wrong",
			wantError: true,
		},
		{
			name:      "empty secret",
			body:      body,
			signature: signature,
			secret:    "",
			wantError: true,
		},
		{
			name:      "missing signature",
			body:      body,
			signature: "",
			secret:    secret,
			wantError: true,
		},
		{
			name:      "malformed signature",
			body:      body,
			signature: strings.Repeat("!", 44),
			secret:    secret,
			wantError: true,
		},
		{
			name:      "wrong signature",
			body:      body,
			signature: "At28OCuRuuPXUwhqVp0fvxX/ePW2UiKjBXYDS9HVSzA=",
			secret:    secret,
			wantError: true,
		},
		{
			name:      "unpadded signature",
			body:      body,
			signature: strings.TrimSuffix(signature, "="),
			secret:    secret,
			wantError: true,
		},
		{
			name:      "newline in signature",
			body:      body,
			signature: signature + "\n",
			secret:    secret,
			wantError: true,
		},
		{
			name:      "empty body",
			body:      "",
			signature: "xegJJdA/wgeubV7y+ZxWJ/hb7PqYLrEZXih5cOQC7n0=",
			secret:    secret,
			wantError: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := castle.VerifyWebhookSignature([]byte(tc.body), tc.signature, tc.secret)
			if tc.wantError {
				if !errors.Is(err, castle.ErrWebhookVerification) {
					t.Fatalf("got %v, want ErrWebhookVerification", err)
				}
				if err.Error() != "castle: webhook signature verification failed" {
					t.Fatalf("unexpected error text: %q", err.Error())
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
