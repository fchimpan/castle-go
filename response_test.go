package castle_test

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fchimpan/castle-go"
	"github.com/fchimpan/castle-go/api"
)

func TestQueryEventsCSV(t *testing.T) {
	t.Parallel()
	const query = `{"filters":[{"field":"type","op":"$eq","value":"$login"}],"columns":["user.id"]}`
	const csv = "user.id\nsynthetic\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "text/csv" {
			t.Error("CSV request content type missing")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		assertJSONEqual(t, body, []byte(query))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		io.WriteString(w, csv)
	}))
	t.Cleanup(server.Close)
	client, err := castle.NewClient("synthetic-secret", castle.Config{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.QueryEventsWithBody(context.Background(), "text/csv", strings.NewReader(query))
	if err != nil {
		t.Fatal(err)
	}
	if string(response.Body) != csv || response.JSON200 != nil || response.ContentType() != "text/csv; charset=utf-8" {
		t.Fatalf("CSV response = %#v", response)
	}
}

func TestDatasetDownloadReturnsLocationWithoutFollowing(t *testing.T) {
	t.Parallel()
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationCalls.Add(1)
	}))
	t.Cleanup(destination.Close)
	location := destination.URL + "/synthetic.gz?signature=synthetic"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, location, http.StatusFound)
	}))
	t.Cleanup(server.Close)
	client, err := castle.NewClient("synthetic-secret", castle.Config{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.DownloadIpDatasetExport(context.Background(), "proxies", "7d", "jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode() != 302 || response.Headers302 == nil || response.Headers302.Location == nil || *response.Headers302.Location != location {
		t.Fatalf("download response = %#v", response)
	}
	if destinationCalls.Load() != 0 {
		t.Fatal("presigned download was automatically followed")
	}
}

func TestDatasetMetadataDecodesHeaders(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s", r.Method)
		}
		w.Header().Set("X-Snapshot-Row-Count", "0")
		w.Header().Set("X-Snapshot-Byte-Size", "1024")
		w.Header().Set("X-Snapshot-Generated-At", "2026-11-05T00:00:00Z")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	client, err := castle.NewClient("synthetic-secret", castle.Config{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.GetLatestIpDatasetExportMetadata(context.Background(), "proxies", "7d", "jsonl")
	if err != nil {
		t.Fatal(err)
	}
	headers := response.Headers200
	if headers == nil || headers.XSnapshotRowCount == nil || *headers.XSnapshotRowCount != 0 || headers.XSnapshotByteSize == nil || *headers.XSnapshotByteSize != 1024 || headers.XSnapshotGeneratedAt == nil || headers.XSnapshotGeneratedAt.Year() != 2026 {
		t.Fatalf("metadata = %#v", headers)
	}
	if headers.XSnapshotExpiresAt != nil {
		t.Fatal("absent expiry header was populated")
	}
}

type failingJSON struct{}

func (failingJSON) MarshalJSON() ([]byte, error) {
	return nil, errors.New("sensitive synthetic-secret")
}

func TestRequestEncodingFailureIsPrivateAndDoesNotSend(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	client, err := castle.NewClient("synthetic-secret", castle.Config{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return nil, errors.New("unexpected request")
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := api.GroupQuery{
		Columns: []api.Column{
			{
				Name:  "Synthetic",
				Field: "user.id",
				Options: new(map[string]any{
					"synthetic": failingJSON{},
				}),
			},
		},
	}
	response, err := client.GroupEvents(context.Background(), request)
	if response != nil || err == nil || strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "synthetic-secret") || errors.Unwrap(err) != nil {
		t.Fatalf("result = %v, error = %v", response, err)
	}
	if calls.Load() != 0 {
		t.Fatal("unencodable request was sent")
	}
}

func TestSharedResponseErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
	}{
		{
			name:        "malformed API error preserves status",
			status:      422,
			contentType: "application/json",
			body:        "sensitive {",
		},
		{
			name:        "typed decode error is private",
			status:      200,
			contentType: "application/json",
			body:        `{"address":"203.0.113.42","type":"ipv4","asn":0,"location":{},"tunnels":[{"type":"vpn","operator":"Synthetic","last_seen_at":"sensitive"}]}`,
		},
		{
			name:   "missing content type",
			status: 200,
			body:   `{}`,
		},
		{
			name:        "unexpected content type",
			status:      200,
			contentType: "text/html",
			body:        `{}`,
		},
		{
			name:        "null JSON",
			status:      200,
			contentType: "application/json",
			body:        "null",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header()["Content-Type"] = []string{tc.contentType}
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			t.Cleanup(server.Close)
			client, err := castle.NewClient("synthetic-secret", castle.Config{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.IpsLookupSingle(context.Background(), "203.0.113.42")
			if response != nil || err == nil || strings.Contains(err.Error(), "sensitive") || errors.Unwrap(err) != nil {
				t.Fatalf("result = %v, error = %v", response, err)
			}
			if tc.status == 422 {
				var apiError *castle.APIError
				if !errors.As(err, &apiError) || apiError.StatusCode != tc.status {
					t.Fatalf("HTTP status lost: %v", err)
				}
			}
		})
	}
}

func TestMetadataHeaderDecodeErrorIsPrivate(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Snapshot-Row-Count", "sensitive synthetic-secret")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	client, err := castle.NewClient("synthetic-secret", castle.Config{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.GetLatestIpDatasetExportMetadata(context.Background(), "proxies", "7d", "jsonl")
	if response != nil || err == nil || strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "synthetic-secret") || errors.Unwrap(err) != nil {
		t.Fatalf("result = %v, error = %v", response, err)
	}
}

func TestJSONMediaTypes(t *testing.T) {
	for _, contentType := range []string{"application/json; charset=utf-8", "Application/JSON"} {
		t.Run(contentType, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				io.WriteString(w, "[]")
			}))
			t.Cleanup(server.Close)
			client, err := castle.NewClient("synthetic-secret", castle.Config{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.GetLists(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if response.JSON200 == nil || len(*response.JSON200) != 0 || response.ContentType() != contentType {
				t.Fatalf("response = %#v", response)
			}
		})
	}
}

func TestConfiguredResponseSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		limit    int64
		accepted bool
	}{
		{
			name:     "at configured limit",
			limit:    2,
			accepted: true,
		},
		{
			name:  "over configured limit",
			limit: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, err := castle.NewClient("synthetic-secret", castle.Config{
				MaxResponseBytes: tc.limit,
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 200,
							Header:     http.Header{"Content-Type": {"application/json"}},
							Body:       io.NopCloser(strings.NewReader("[]")),
						}, nil
					}),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.GetLists(context.Background())
			if (err == nil) != tc.accepted || (response != nil) != tc.accepted {
				t.Fatalf("result = %v, error = %v", response, err)
			}
		})
	}
}

func TestInvalidResponseSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int64
	}{
		{
			name:  "negative",
			limit: -1,
		},
		{
			name:  "overflow",
			limit: math.MaxInt64,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := castle.NewClient("synthetic-secret", castle.Config{MaxResponseBytes: tc.limit}); err == nil {
				t.Fatal("invalid response size limit accepted")
			}
		})
	}
}
