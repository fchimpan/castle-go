# castle-go

An unofficial Go SDK for [Castle](https://castle.io). Requires Go 1.27.3 or later.

The SDK covers all 27 operations in the checked-in official specification,
including Risk, Filter, Lists and List Items, Privacy, Events, IP Intelligence,
and IP dataset downloads. Deprecated Privacy operations are included.
Request models, response models, and endpoint calls are generated from Castle's
[official OpenAPI definition](https://reference.castle.io/v1/api.json).

## Install

```sh
go get github.com/fchimpan/castle-go@v0.0.1
```

## Assess an event

Import the root client and the generated models:

```go
import (
	"context"

	castle "github.com/fchimpan/castle-go"
	"github.com/fchimpan/castle-go/api"
)

func assessLogin(ctx context.Context, secret, token string, origin api.Context) (*api.RiskResponse, error) {
	client, err := castle.NewClient(secret, castle.Config{})
	if err != nil {
		return nil, err
	}

	var event api.Risk
	if err := event.FromLoginRiskRequest(api.LoginRiskRequest{
		Context:      &origin,
		RequestToken: new(token),
		Status:       api.LoginRiskRequestStatusSucceeded,
		User: api.UserForMonitorAndRisk{
			Id: "user-123",
		},
	}); err != nil {
		return nil, err
	}
	return client.Risk(ctx, event)
}
```

Reuse a client across requests. Supply the request token from Castle's client SDK
and the original request's IP address and ordered headers in `api.Context`.
The generated header scalar union supports strings, booleans, and integers.
See [the executable example](example_test.go) for header construction and result handling.

The generated `From...RiskRequest` methods set the event discriminator. Risk
supports challenge, registration, login, profile update, profile reset,
transaction, logout, and custom events. Set status explicitly. The generated
models do not apply server defaults or validate every schema constraint.

Each method returns a generated response envelope. Read `JSON200` or `JSON201`
for the typed payload, `Body` for the original response bytes, and `StatusCode()`
for the status. For Risk and Filter, use `result.JSON201.ValueByDiscriminator()`
or `result.JSON201.AsLoginResponse()` to read an assessment.
Check whether `Scores` is present before reading a score. An absent
score is different from a score of zero. The response union preserves unknown
JSON fields; a typed projection can discard fields outside its schema.

## Call other APIs

Use the same client for every operation. For example:

```go
lists, err := client.GetLists(ctx)
if err != nil {
	return err
}
for _, list := range *lists.JSON200 {
	fmt.Println(list.Id, list.Name)
}
```

Methods use the OpenAPI operation names with an initial capital, such as
`CreateList`, `QueryListItems`, `GetUserPrivacy`, `QueryEvents`, and
`IpsLookupBatch`. Their request and response types live in `api`.
Bodyless successes, including Privacy's 202 and deletion's 204, return an
envelope with the status and headers.

`QueryEventsWithBody(ctx, "text/csv", reader)` supports the documented CSV
mode. The official example sends a JSON query with that content type. Read the
CSV result from `Body`; `JSON200` is nil for CSV responses. Other `WithBody`
variants accept a content type and reader and use the same client policies.

`GetLatestIpDatasetExportMetadata` sends HEAD and returns typed metadata in
`Headers200`. `DownloadIpDatasetExport` returns the documented 302 response
and presigned URL in `Headers302.Location`. Fetch that URL separately with an
HTTP client that does not attach the Castle API secret. The SDK does not follow
the redirect or download the dataset automatically.

## Configure requests and handle failures

`castle.Config` accepts a `BaseURL` and an `HTTPClient`. Empty configuration uses
`https://api.castle.io/v1`. The SDK copies the HTTP client and disables redirects
on the copy. It uses HTTP Basic authentication with an empty username and the
secret as the password. Custom transports remain responsible for their own
behavior.

The default HTTP client has no timeout. Set a context deadline or configure
`HTTPClient.Timeout` to bound the request duration. Successful response bodies
are limited to 4 MiB by default. Set `Config.MaxResponseBytes` to a positive byte
limit for larger responses, such as Events queries or CSV exports.

The root client accepts each operation's documented success statuses and content
types. JSON responses must declare `application/json`; a missing or unexpected
content type is an error. The SDK adds no retries and does not replace API failures with
an allow decision. Use `errors.As` to inspect `*castle.APIError.StatusCode`.
HTTP status is available even for empty or malformed error bodies. Error messages
exclude response bodies, request data, and API secrets. Use `errors.Is` with
`context.Canceled` and `context.DeadlineExceeded` to detect cancellation.

Set `SkipRequestTokenValidation: new(true)` or
`SkipContextValidation: new(true)` only when the corresponding data is unavailable.
These flags allow the corresponding fields to be omitted. Skipping request token
validation prevents risk scores from being calculated.

Custom-event status and nullable changeset fields use
`github.com/oapi-codegen/nullable`. Use an explicit null value when sending JSON
`null`; leave an optional nullable field unspecified to omit it. Request
`Properties` has an extra object level compared with response `Properties`, as
declared in the official schema.

The generated `api.Client` and `api.ClientWithResponses` are low-level clients. Use the root `castle.Client`
for the authentication, redirect, and error behavior described here.

## Regenerate and test

```sh
go generate ./...
go test ./...
go vet ./...
go test -race ./...
```

Generation uses the checked-in schema and pinned oapi-codegen v2.8.2. It does not
fetch a newer schema or select a newer tool version. Tests use synthetic data and
local HTTP servers, without Castle credentials. CI also checks for stale generated
code in a separate job. Tests run with the race detector enabled, and `go vet`
runs in its own job. Read [the testing guide](TESTING.md) for test design and
[the generation guide](api/GENERATION.md) before updating the snapshot.

All 27 operations have local HTTP fixtures that check requests and decoded
responses. These tests do not verify live Castle accounts or service behavior.

## Migration from the unreleased Risk-only prototype

`Client.Risk` now returns `*api.RiskResponse`, matching every other operation's
response envelope. Change `result.AsLoginResponse()` to
`result.JSON201.AsLoginResponse()` and similarly move other assessment access
through `JSON201`. Request models remain generated. Some newly named component
types replace anonymous nested schemas; regenerate and compile callers when
updating the specification.

Releases remain in the `v0.x.y` series. New features and breaking public API changes
use a minor version increment; backward-compatible fixes use a patch increment.
