package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/fchimpan/castle-go/api"
	"github.com/oapi-codegen/nullable"
)

func TestRiskDiscriminators(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		kind      string
		extra     string
		construct func([]byte, *api.Risk) error
	}{
		{
			name:  "challenge",
			kind:  "$challenge",
			extra: `,"challenge":{"trigger_event":{"type":"$login","status":"$attempted"}},"authentication_method":{"type":"$email"}`,
			construct: func(data []byte, r *api.Risk) error {
				var v api.ChallengeRiskRequest
				if err := json.Unmarshal(data, &v); err != nil {
					return err
				}
				return r.FromChallengeRiskRequest(v)
			},
		},
		{
			name: "registration",
			kind: "$registration",
			construct: func(data []byte, r *api.Risk) error {
				var v api.RegistrationRiskRequest
				if err := json.Unmarshal(data, &v); err != nil {
					return err
				}
				return r.FromRegistrationRiskRequest(v)
			},
		},
		{
			name: "login",
			kind: "$login",
			construct: func(data []byte, r *api.Risk) error {
				var v api.LoginRiskRequest
				if err := json.Unmarshal(data, &v); err != nil {
					return err
				}
				return r.FromLoginRiskRequest(v)
			},
		},
		{
			name:  "profile update",
			kind:  "$profile_update",
			extra: `,"changeset":{"password":{"changed":true}}`,
			construct: func(data []byte, r *api.Risk) error {
				var v api.ProfileUpdateRiskRequest
				if err := json.Unmarshal(data, &v); err != nil {
					return err
				}
				return r.FromProfileUpdateRiskRequest(v)
			},
		},
		{
			name:  "profile reset",
			kind:  "$profile_reset",
			extra: `,"changeset":{"email":{"from":"old@example.com","to":null}}`,
			construct: func(data []byte, r *api.Risk) error {
				var v api.ProfileResetRiskRequest
				if err := json.Unmarshal(data, &v); err != nil {
					return err
				}
				return r.FromProfileResetRiskRequest(v)
			},
		},
		{
			name:  "transaction",
			kind:  "$transaction",
			extra: `,"transaction":{"id":"transaction-1","type":"$purchase","amount":{"type":"$fiat","currency":"USD","value":"12.50"}}`,
			construct: func(data []byte, r *api.Risk) error {
				var v api.TransactionRiskRequest
				if err := json.Unmarshal(data, &v); err != nil {
					return err
				}
				return r.FromTransactionRiskRequest(v)
			},
		},
		{
			name: "logout",
			kind: "$logout",
			construct: func(data []byte, r *api.Risk) error {
				var v api.LogoutRiskRequest
				if err := json.Unmarshal(data, &v); err != nil {
					return err
				}
				return r.FromLogoutRiskRequest(v)
			},
		},
		{
			name:  "custom",
			kind:  "$custom",
			extra: `,"name":"support_action"`,
			construct: func(data []byte, r *api.Risk) error {
				var v api.CustomRiskRequest
				if err := json.Unmarshal(data, &v); err != nil {
					return err
				}
				return r.FromCustomRiskRequest(v)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(`{"status":"$succeeded","user":{"id":"synthetic"},"skip_context_validation":true,"skip_request_token_validation":true` + tc.extra + `}`)
			var r api.Risk
			if err := tc.construct(data, &r); err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if got["type"] != tc.kind || got["status"] != "$succeeded" || got["user"].(map[string]any)["id"] != "synthetic" || got["skip_context_validation"] != true || got["skip_request_token_validation"] != true {
				t.Fatalf("unexpected request: %s", b)
			}
			var expected map[string]any
			if err := json.Unmarshal(data, &expected); err != nil {
				t.Fatal(err)
			}
			expected["type"] = tc.kind
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("request = %s; expected fields %v", b, expected)
			}
			if _, err := r.ValueByDiscriminator(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGeneratedWireValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "conditional omission",
			input: `{"type":"$login","status":"$succeeded","user":{"id":"synthetic"},"skip_context_validation":true,"skip_request_token_validation":true}`,
		},
		{
			name:  "nested properties and headers",
			input: `{"type":"$login","status":"$succeeded","user":{"id":"synthetic"},"request_token":"synthetic-token","skip_context_validation":false,"context":{"ip":"192.0.2.1","headers":[["X-String","value"],["X-Bool",false],["X-Int",0]]},"properties":{"outer":{"flag":false,"count":0}}}`,
		},
		{
			name:  "custom explicit null",
			input: `{"type":"$custom","status":null,"user":{"id":"synthetic"},"skip_context_validation":true,"skip_request_token_validation":true}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var r api.Risk
			if err := json.Unmarshal([]byte(tc.input), &r); err != nil {
				t.Fatal(err)
			}
			v, err := r.ValueByDiscriminator()
			if err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			var got, expected any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.input), &expected); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("JSON = %s, want %s", b, tc.input)
			}

		})
	}
}

func TestChangesetNullAndCustomKeys(t *testing.T) {
	t.Parallel()
	var c api.Changeset
	input := `{"authentication_method.type":{"from":null},"email":{"changed":true},"custom":{"from":null,"to":"new"}}`
	if err := json.Unmarshal([]byte(input), &c); err != nil {
		t.Fatal(err)
	}
	v, err := c.AuthenticationMethodType.AsChangesetAuthenticationMethodType1()
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"from":null}` {
		t.Fatalf("null or omission lost: %s", b)
	}
	changed, err := c.Email.AsChangedChangesetEntry()
	if err != nil || !bool(changed.Changed) {
		t.Fatalf("changed email = %#v, %v", changed, err)
	}
	custom, err := c.AdditionalProperties["custom"].AsChangesetEntry()
	if err != nil || !custom.From.IsNull() || custom.To.MustGet() != "new" {
		t.Fatalf("custom changeset = %#v, %v", custom, err)
	}
	b, err = json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"custom":{"from":null,"to":"new"}`)) {
		t.Fatalf("custom lost: %s", b)
	}
}

func TestResponseScoresAndUnknownFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		scores  string
		present bool
	}{
		{
			name:    "omitted",
			scores:  "",
			present: false,
		},
		{
			name:    "zero",
			scores:  `,"scores":{"bot":{"score":0},"account_abuse":{"score":0},"account_takeover":{"score":0}}`,
			present: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := `{"type":"$login","policy":{"action":null,"name":null,"id":null,"revision_id":null},"signals":{},"device":{"fingerprint":null},"future":{"x":1}` + tc.scores + `}`
			var r api.FilterAndRiskResponse
			if err := json.Unmarshal([]byte(input), &r); err != nil {
				t.Fatal(err)
			}
			v, err := r.AsLoginResponse()
			if err != nil {
				t.Fatal(err)
			}
			if (v.Scores != nil) != tc.present {
				t.Fatalf("scores present = %v", v.Scores != nil)
			}
			if tc.present && (v.Scores.Bot.Score == nil || *v.Scores.Bot.Score != 0) {
				t.Fatal("zero score lost")
			}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(b, []byte(`"future"`)) {
				t.Fatalf("unknown field lost: %s", b)
			}
		})
	}
}

func TestResponseDiscriminators(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		kind    string
		matches func(any) bool
	}{
		{
			name:    "challenge",
			kind:    "$challenge",
			matches: func(value any) bool { _, ok := value.(api.ChallengeResponse); return ok },
		},
		{
			name:    "registration",
			kind:    "$registration",
			matches: func(value any) bool { _, ok := value.(api.RegistrationResponse); return ok },
		},
		{
			name:    "login",
			kind:    "$login",
			matches: func(value any) bool { _, ok := value.(api.LoginResponse); return ok },
		},
		{
			name:    "profile update",
			kind:    "$profile_update",
			matches: func(value any) bool { _, ok := value.(api.ProfileUpdateResponse); return ok },
		},
		{
			name:    "profile reset",
			kind:    "$profile_reset",
			matches: func(value any) bool { _, ok := value.(api.ProfileResetResponse); return ok },
		},
		{
			name:    "transaction",
			kind:    "$transaction",
			matches: func(value any) bool { _, ok := value.(api.TransactionResponse); return ok },
		},
		{
			name:    "logout",
			kind:    "$logout",
			matches: func(value any) bool { _, ok := value.(api.LogoutResponse); return ok },
		},
		{
			name:    "custom",
			kind:    "$custom",
			matches: func(value any) bool { _, ok := value.(api.CustomResponse); return ok },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := fmt.Appendf(nil, `{"type":%q,"policy":{"action":null,"name":null,"id":null,"revision_id":null},"signals":{},"device":{"fingerprint":null},"future":{"expanded":true}}`, tc.kind)
			var r api.FilterAndRiskResponse
			if err := json.Unmarshal(data, &r); err != nil {
				t.Fatal(err)
			}
			v, err := r.ValueByDiscriminator()
			if err != nil {
				t.Fatal(err)
			}
			if !tc.matches(v) {
				t.Fatalf("wrong public response variant for %s", tc.kind)
			}
			projected, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(projected, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["type"] != tc.kind {
				t.Fatalf("projection = %s", projected)
			}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(b, []byte(`"future":{"expanded":true}`)) {
				t.Fatalf("unknown expansion lost: %s", b)
			}
		})
	}
}

func TestAuthenticationChangesetStates(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		value    api.ChangesetAuthenticationMethodType1
		expected string
	}{
		{
			name:     "omitted",
			expected: `{}`,
		},
		{
			name: "null from and omitted to",
			value: api.ChangesetAuthenticationMethodType1{
				From: nullable.NewNullNullable[api.ChangesetAuthenticationMethodType1From](),
			},
			expected: `{"from":null}`,
		},
		{
			name: "value from and null to",
			value: api.ChangesetAuthenticationMethodType1{
				From: nullable.NewNullableWithValue(api.ChangesetAuthenticationMethodType1From("$email")),
				To:   nullable.NewNullNullable[api.ChangesetAuthenticationMethodType1To](),
			},
			expected: `{"from":"$email","to":null}`,
		},
		{
			name: "null from and value to",
			value: api.ChangesetAuthenticationMethodType1{
				From: nullable.NewNullNullable[api.ChangesetAuthenticationMethodType1From](),
				To:   nullable.NewNullableWithValue(api.ChangesetAuthenticationMethodType1To("$authenticator")),
			},
			expected: `{"from":null,"to":"$authenticator"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var union api.Changeset_AuthenticationMethodType
			if err := union.FromChangesetAuthenticationMethodType1(tc.value); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(union)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.expected), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("JSON = %s, want %s", data, tc.expected)
			}
			projected, err := union.AsChangesetAuthenticationMethodType1()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(projected, tc.value) {
				t.Fatalf("projection = %#v, want %#v", projected, tc.value)
			}
		})
	}
}

func TestChangesetKnownAndCustomShapes(t *testing.T) {
	t.Parallel()
	const fixture = `{"password":{"changed":true},"email":{"from":"old@example.com","to":null},"phone":{"from":null,"to":"+12025550123"},"name":{"from":null,"to":"Example"},"custom_changed":{"changed":true},"custom_values":{"from":"old","to":null}}`
	var value api.Changeset
	if err := json.Unmarshal([]byte(fixture), &value); err != nil {
		t.Fatal(err)
	}
	email, err := value.Email.AsChangesetEmail1()
	if err != nil || email.From != "old@example.com" || !email.To.IsNull() {
		t.Fatalf("email = %#v, %v", email, err)
	}
	phone, err := value.Phone.AsChangesetPhone1()
	if err != nil || !phone.From.IsNull() || phone.To.MustGet() != "+12025550123" {
		t.Fatalf("phone = %#v, %v", phone, err)
	}
	name, err := value.Name.AsChangesetEntry()
	if err != nil || !name.From.IsNull() || name.To.MustGet() != "Example" {
		t.Fatalf("name = %#v, %v", name, err)
	}
	changed, err := value.AdditionalProperties["custom_changed"].AsChangedChangesetEntry()
	if err != nil || !bool(changed.Changed) || value.Password == nil || !bool(value.Password.Changed) {
		t.Fatal("changed flags lost")
	}
	custom, err := value.AdditionalProperties["custom_values"].AsChangesetEntry()
	if err != nil || custom.From.MustGet() != "old" || !custom.To.IsNull() {
		t.Fatalf("custom = %#v, %v", custom, err)
	}
	actual, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(actual, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(fixture), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON = %s", actual)
	}
}

func TestExpandedResponse(t *testing.T) {
	t.Parallel()
	const fixture = `{
 "type":"$login","status":"$succeeded","created_at":"2026-11-03T12:00:00Z",
 "policy":{"action":"challenge","id":null,"name":null,"revision_id":null},
 "signals":{"bot_behavior":{}},"authenticated":false,
 "device":{"fingerprint":null,"timezone":{"offset":0,"name":null},"screen":{"orientation":null}},
 "email":{"disposable":false,"domain_details":{"spf_record":{"exists":false},"dmarc_record":{"exists":null}}},
 "behavior":{"fingerprint":null},"properties":{"verified":false,"count":0},
 "agent":{"category":"ai_agent","identified_by":["web_bot_auth"],"name":"Synthetic","operator":"example"},
 "future":{"retained":true}
 }`
	var result api.FilterAndRiskResponse
	if err := json.Unmarshal([]byte(fixture), &result); err != nil {
		t.Fatal(err)
	}
	login, err := result.AsLoginResponse()
	if err != nil {
		t.Fatal(err)
	}
	if login.Authenticated == nil || *login.Authenticated || login.CreatedAt == nil || login.CreatedAt.Year() != 2026 {
		t.Fatal("expanded scalars lost")
	}
	if !login.Device.Fingerprint.IsNull() || login.Device.Timezone == nil || login.Device.Timezone.Offset.MustGet() != 0 || !login.Device.Timezone.Name.IsNull() {
		t.Fatal("nullable device fields lost")
	}
	if login.Device.Screen == nil || !login.Device.Screen.Orientation.IsNull() || login.Device.Screen.Screen.IsSpecified() {
		t.Fatal("null and omission conflated")
	}
	if login.Email == nil || login.Email.Disposable == nil || *login.Email.Disposable || login.Email.DomainDetails == nil {
		t.Fatal("email fields lost")
	}
	if login.Email.DomainDetails.SpfRecord.Exists.MustGet() || !login.Email.DomainDetails.DmarcRecord.Exists.IsNull() {
		t.Fatal("nullable booleans lost")
	}
	if login.Agent == nil || login.Agent.Name != "Synthetic" || login.Behavior == nil || !login.Behavior.Fingerprint.IsNull() {
		t.Fatal("expanded objects lost")
	}
	if login.Policy.Action.MustGet() != "challenge" {
		t.Fatal("policy action lost")
	}
	data, err := json.Marshal(result)
	if err != nil || !bytes.Contains(data, []byte(`"future"`)) {
		t.Fatalf("raw response = %s, %v", data, err)
	}
}
