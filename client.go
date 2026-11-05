// Package castle provides a client for the Castle API.
package castle

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/fchimpan/castle-go/api"
)

const defaultBaseURL = "https://api.castle.io/v1"

// Config configures the client. Its zero value uses Castle's production API.
type Config struct {
	// BaseURL includes the API version path. Empty uses https://api.castle.io/v1.
	BaseURL string
	// HTTPClient is copied before use. Nil uses a client without a timeout.
	HTTPClient *http.Client
	// MaxResponseBytes bounds response bodies. Zero uses 4 MiB.
	MaxResponseBytes int64
}

// Client calls Castle's generated API operations. It is safe for concurrent use.
type Client struct {
	api              *api.Client
	maxResponseBytes int64
}

// NewClient constructs a client using apiSecret as the Basic authentication password.
func NewClient(apiSecret string, config Config) (*Client, error) {
	if apiSecret == "" {
		return nil, errors.New("castle: API secret is required")
	}
	maxResponseBytes := config.MaxResponseBytes
	if maxResponseBytes < 0 || maxResponseBytes == math.MaxInt64 {
		return nil, errors.New("castle: invalid maximum response size")
	}
	if maxResponseBytes == 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}
	base := config.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil || u == nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(base, "#") || u.Opaque != "" {
		return nil, errors.New("castle: invalid base URL")
	}
	httpClient := http.Client{}
	if config.HTTPClient != nil {
		httpClient = *config.HTTPClient
	}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	generated, err := api.NewClient(base, api.WithHTTPClient(&httpClient), api.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.SetBasicAuth("", apiSecret)
		return nil
	}))
	if err != nil {
		return nil, errors.New("castle: could not initialize client")
	}
	return &Client{api: generated, maxResponseBytes: maxResponseBytes}, nil
}
