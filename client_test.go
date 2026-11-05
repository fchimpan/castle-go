package castle_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fchimpan/castle-go"
	"github.com/fchimpan/castle-go/api"
)

func event(t *testing.T) api.Risk {
	t.Helper()
	var v api.Risk
	if err := json.Unmarshal([]byte(`{"type":"$login","status":"$succeeded","user":{"id":"synthetic"},"skip_context_validation":true,"skip_request_token_validation":true}`), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRiskSendsAuthenticatedAssessmentAndReturnsPolicy(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "" || pass != "secret" {
			t.Error("expected empty username and configured secret")
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/risk" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("request is not declared as JSON")
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request["type"] != "$login" || request["status"] != "$succeeded" {
			t.Errorf("event = %v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"type":"$login","policy":{"action":"challenge","id":null,"name":null,"revision_id":null},"signals":{},"device":{"fingerprint":null},"future_field":"kept"}`)
	}))
	t.Cleanup(server.Close)
	client, err := castle.NewClient("secret", castle.Config{
		BaseURL:    server.URL + "/v1",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Risk(context.Background(), event(t))

	if err != nil {
		t.Fatal(err)
	}
	login, err := result.JSON201.AsLoginResponse()
	if err != nil {
		t.Fatal(err)
	}
	action, err := login.Policy.Action.Get()
	if err != nil || action != api.PolicyActionChallenge {
		t.Fatalf("action = %q, error = %v", action, err)
	}
	raw, err := json.Marshal(result.JSON201)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["future_field"] != "kept" {
		t.Fatal("unknown response data lost")
	}
}

func TestRiskHTTPFailures(t *testing.T) {
	cases := []riskFailure{
		{
			name:     "unauthorized",
			status:   401,
			body:     "sensitive",
			apiError: true,
		},
		{
			name:     "forbidden",
			status:   403,
			body:     "",
			apiError: true,
		},
		{
			name:     "invalid",
			status:   422,
			body:     "{",
			apiError: true,
		},
		{
			name:     "documented validation error",
			status:   422,
			body:     `{"type":"record_invalid","message":"sensitive request value"}`,
			apiError: true,
		},
		{
			name:     "limited",
			status:   429,
			body:     "{}",
			apiError: true,
		},
		{
			name:     "server",
			status:   500,
			body:     "secret",
			apiError: true,
		},
		{
			name:     "unexpected",
			status:   200,
			body:     "{}",
			apiError: true,
		},
	}
	testRiskFailures(t, cases)
}

func TestRiskInvalidSuccessResponses(t *testing.T) {
	cases := []riskFailure{
		{
			name:   "empty",
			status: 201,
			body:   "",
		},
		{
			name:   "null",
			status: 201,
			body:   "null",
		},
		{
			name:   "scalar",
			status: 201,
			body:   "42",
		},
		{
			name:   "missing assessment",
			status: 201,
			body:   `{"type":"$login"}`,
		},
		{
			name:   "unknown type",
			status: 201,
			body:   `{"type":"other","policy":{"action":"allow","id":null,"name":null,"revision_id":null},"signals":{},"device":{"fingerprint":null}}`,
		},
		{
			name:   "invalid typed field",
			status: 201,
			body:   `{"type":"$login","policy":{"action":"allow","id":null,"name":null,"revision_id":null},"signals":{},"device":{"fingerprint":null},"created_at":42}`,
		},
	}
	testRiskFailures(t, cases)
}

type riskFailure struct {
	name     string
	status   int
	body     string
	apiError bool
}

func testRiskFailures(t *testing.T, cases []riskFailure) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			t.Cleanup(s.Close)
			c, err := castle.NewClient("secret", castle.Config{
				BaseURL:    s.URL,
				HTTPClient: s.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := c.Risk(context.Background(), event(t))
			if result != nil {
				t.Fatalf("result on failure: %#v", result)
			}
			if calls.Load() != 1 {
				t.Fatalf("assessment sent %d times", calls.Load())
			}
			if err == nil {
				t.Fatal("expected error")
			}
			var apiErr *castle.APIError
			if errors.As(err, &apiErr) != tc.apiError {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.apiError && apiErr.StatusCode != tc.status {
				t.Fatalf("status %d", apiErr.StatusCode)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("unsafe error: %v", err)
			}
		})
	}
}

func TestRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, cross := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/cross=%v", status, cross), func(t *testing.T) {
				t.Parallel()
				destination := make(chan struct{}, 1)
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destination <- struct{}{} }))
				t.Cleanup(target.Close)
				var server *httptest.Server
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/destination" {
						destination <- struct{}{}
						return
					}
					location := "/destination"
					if cross {
						location = target.URL
					}
					http.Redirect(w, r, location, status)
				}))
				t.Cleanup(server.Close)
				original := server.Client()
				original.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
				c, err := castle.NewClient("secret", castle.Config{
					BaseURL:    server.URL,
					HTTPClient: original,
				})
				if err != nil {
					t.Fatal(err)
				}
				_, err = c.Risk(context.Background(), event(t))
				var apiErr *castle.APIError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
					t.Fatalf("error: %v", err)
				}
				select {
				case <-destination:
					t.Fatal("redirect followed")
				default:
				}
				if err := original.CheckRedirect(nil, nil); err != nil {
					t.Fatal("original client mutated")
				}
			})
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func validResponseOfSize(size int) string {
	const prefix = `{"type":"$login","policy":{"action":"allow","id":null,"name":null,"revision_id":null},"signals":{},"device":{"fingerprint":null},"padding":"`
	const suffix = `"}`
	return prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix
}

func TestResponseSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		size     int
		accepted bool
	}{
		{
			name:     "at limit",
			size:     4 * 1024 * 1024,
			accepted: true,
		},
		{
			name: "over limit",
			size: 4*1024*1024 + 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 201, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(validResponseOfSize(tc.size)))}, nil
			})}
			c, err := castle.NewClient("secret", castle.Config{HTTPClient: h})
			if err != nil {
				t.Fatal(err)
			}
			result, err := c.Risk(context.Background(), event(t))
			if (err == nil) != tc.accepted || (result != nil) != tc.accepted {
				t.Fatalf("result = %v, error = %v", result != nil, err)
			}
		})
	}
}

func TestResponseBodyClosed(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		wantSuccess bool
	}{
		{
			name:        "success",
			wantSuccess: true,
			status:      201,
			body:        `{"type":"$login","policy":{"action":"allow","id":null,"name":null,"revision_id":null},"signals":{},"device":{"fingerprint":null}}`,
		},
		{
			name:   "invalid",
			status: 201,
			body:   "{",
		},
		{
			name:   "too large",
			status: 201,
			body:   validResponseOfSize(4*1024*1024 + 1),
		},
		{
			name:   "failure",
			status: 422,
			body:   "sensitive",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &trackedBody{Reader: strings.NewReader(tc.body)}
			h := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": {"application/json"}}, Body: body}, nil
			})}
			c, err := castle.NewClient("secret", castle.Config{HTTPClient: h})
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Risk(context.Background(), event(t))
			if (err == nil) != tc.wantSuccess {
				t.Fatalf("error: %v", err)
			}
			if !body.closed {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestSafeTransportAndContext(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause error
	}{
		{
			name:  "secret",
			cause: errors.New("secret body"),
		},
		{
			name:  "canceled",
			cause: context.Canceled,
		},
		{
			name:  "deadline",
			cause: context.DeadlineExceeded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, tc.cause })}
			c, err := castle.NewClient("secret", castle.Config{HTTPClient: h})
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Risk(context.Background(), event(t))
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("error: %v", err)
			}
			if tc.cause == context.Canceled || tc.cause == context.DeadlineExceeded {
				if !errors.Is(err, tc.cause) {
					t.Fatalf("missing cause: %v", err)
				}
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		duringBody bool
		deadline   bool
	}{
		{
			name: "cancel before response",
		},
		{
			name:       "cancel while reading body",
			duringBody: true,
		},
		{
			name:     "deadline before response",
			deadline: true,
		},
		{
			name:       "deadline while reading body",
			duringBody: true,
			deadline:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			arrived := make(chan struct{})
			release := make(chan struct{})

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				if tc.duringBody {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					io.WriteString(w, `{"type":`)
					w.(http.Flusher).Flush()
				}
				close(arrived)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			t.Cleanup(func() { close(release); server.Close() })
			c, err := castle.NewClient("secret", castle.Config{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			result := make(chan error, 1)
			request := event(t)
			go func() {
				_, err := c.Risk(ctx, request)
				result <- err
			}()
			select {
			case <-arrived:
			case err := <-result:
				t.Fatalf("request did not reach fixture: %v", err)
			}
			want := context.DeadlineExceeded
			if !tc.deadline {
				cancel()
				want = context.Canceled
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("error = %v, want %v", err, want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("request did not return after cancellation")
			}
		})
	}
}

func TestBaseURL(t *testing.T) {
	for _, value := range []string{":", "ftp://host", "https://user:secret@host", "https://host?secret", "https://host#secret", "https://host?"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			_, err := castle.NewClient("secret", castle.Config{BaseURL: value})
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestDefaultEndpointAndClientSettings(t *testing.T) {
	t.Parallel()
	calls := 0
	original := &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.String() != "https://api.castle.io/v1/risk" {
				t.Errorf("URL = %s", r.URL)
			}
			if _, ok := r.Context().Deadline(); !ok {
				t.Error("injected client timeout was lost")
			}
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	client, err := castle.NewClient("secret", castle.Config{HTTPClient: original})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Risk(context.Background(), event(t))
	var apiErr *castle.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	if original.CheckRedirect != nil || original.Timeout != time.Second {
		t.Fatal("caller client changed")
	}
}

func TestEmptySecret(t *testing.T) {
	t.Parallel()
	if _, err := castle.NewClient("", castle.Config{}); err == nil {
		t.Fatal("empty API secret accepted")
	}
}
