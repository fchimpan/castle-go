package api_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/fchimpan/castle-go/api"
)

func TestFilterRequestVariants(t *testing.T) {
	for _, tc := range []struct {
		kind   string
		status string
		extra  string
	}{
		{
			kind:   "$login",
			status: "$attempted",
		},
		{
			kind:   "$registration",
			status: "$attempted",
		},
		{
			kind:   "$password_reset_request",
			status: "$attempted",
		},
		{
			kind:   "$custom",
			status: "$attempted",
			extra:  `,"name":"support_action"`,
		},
		{
			kind:   "$challenge",
			status: "$requested",
			extra:  `,"challenge":{"trigger_event":{"type":"$login","status":"$attempted"}},"authentication_method":{"type":"$email"}`,
		},
		{
			kind:   "$transaction",
			status: "$attempted",
			extra:  `,"transaction":{"id":"transaction-1","type":"$purchase","amount":{"type":"$fiat","currency":"USD","value":"12.50"}}`,
		},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			t.Parallel()
			input := []byte(`{"type":"` + tc.kind + `","status":"` + tc.status + `","matching_user_id":"synthetic","params":{"email":"synthetic@example.invalid"},"skip_context_validation":true,"skip_request_token_validation":true` + tc.extra + `}`)
			var request api.Filter
			if err := json.Unmarshal(input, &request); err != nil {
				t.Fatal(err)
			}
			event, err := request.ValueByDiscriminator()
			if err != nil {
				t.Fatal(err)
			}
			output, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(input, &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("filter request = %s, want %s", output, input)
			}
		})
	}
}

func TestFilterStatusOmissionAndNull(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{
			name:  "login status omitted",
			input: `{"type":"$login","skip_context_validation":true,"skip_request_token_validation":true}`,
		},
		{
			name:  "custom status null",
			input: `{"type":"$custom","status":null,"skip_context_validation":true,"skip_request_token_validation":true}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var request api.Filter
			if err := json.Unmarshal([]byte(tc.input), &request); err != nil {
				t.Fatal(err)
			}
			value, err := request.ValueByDiscriminator()
			if err != nil {
				t.Fatal(err)
			}
			output, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.input), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("request = %s, want %s", output, tc.input)
			}
		})
	}
}
