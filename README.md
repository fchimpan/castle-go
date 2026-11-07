# castle-go

An unofficial Go SDK for [Castle](https://castle.io). Requires Go 1.27.3 or later.

Request models, response models, and endpoint calls are generated from Castle's
[official OpenAPI definition](https://reference.castle.io/v1/api.json), with shared
authentication, configuration, and error handling.

## Install

```sh
go get github.com/fchimpan/castle-go@latest
```

## Use the client

Set `CASTLE_API_SECRET` to your API secret. This example fetches your lists and
prints their IDs and names:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	castle "github.com/fchimpan/castle-go"
)

func main() {
	client, err := castle.NewClient(os.Getenv("CASTLE_API_SECRET"), castle.Config{})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := client.GetLists(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, list := range *result.JSON200 {
		fmt.Println(list.Id, list.Name)
	}
}
```

Save the code as `main.go` in your application and run `go run .`.
Reuse the client across requests in a long-running application.
Methods follow the OpenAPI operation names with an initial capital. Request and
response types live in the `api` package. See the [generated client](endpoints.gen.go)
for available methods and the [generated models](api/client.gen.go) for their types.

For an assessment example with request models, context headers, and policy
handling, see [the executable example](example_test.go). Supply the request token
from Castle's client SDK and context from the originating request. The generated
models do not apply server defaults or validate every schema constraint.

## Read responses

Each method returns a generated response envelope. Read `JSON200` or `JSON201`
for the typed payload, `Body` for the original response bytes, and `StatusCode()`
for the status. For Risk and Filter, use `result.JSON201.ValueByDiscriminator()`
or `result.JSON201.AsLoginResponse()` to read an assessment.
Check whether `Scores` is present before reading a score. An absent
score is different from a score of zero. The response union preserves unknown
JSON fields; a typed projection can discard fields outside its schema.

Bodyless successes return an envelope with the status and headers.

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

## Verify webhooks

Use the exact raw request body and the `X-Castle-Signature` header before
parsing or processing a webhook:

```go
err := castle.VerifyWebhookSignature(rawBody, r.Header.Get("X-Castle-Signature"), apiSecret)
if errors.Is(err, castle.ErrWebhookVerification) {
    http.Error(w, "Invalid webhook signature", http.StatusBadRequest)
    return
}
```

Read `rawBody` with a bounded reader such as `http.MaxBytesReader`, and reject
read errors before verification. Use the API secret selected for webhook signing
in Castle. Do not reserialize JSON before checking the signature.
Following [Castle's webhook specification](https://docs.castle.io/docs/webhooks),
verification compares the standard Base64 HMAC-SHA256 signature in constant time.
Empty secrets and missing, malformed, or mismatched signatures return
`ErrWebhookVerification`, without including sensitive values. Verification does
not parse the payload or prevent replay of a previously signed delivery.

## Regenerate and test

```sh
go generate ./...
go test -race -count=1 ./...
go vet ./...
```

Generation uses the checked-in schema and pinned oapi-codegen version. It does not
fetch a newer schema or select a newer tool version. Tests use synthetic data and
local HTTP servers, without Castle credentials. CI also checks for stale generated
code in a separate job. Tests run with the race detector enabled, and `go vet`
runs in its own job. Read [the testing guide](TESTING.md) for test design and
[the generation guide](api/GENERATION.md) before updating the snapshot.

Local HTTP fixtures check requests and decoded responses against the checked-in
specification. These tests do not verify live Castle accounts or service behavior.

## Migration from the unreleased Risk-only prototype

`Client.Risk` now returns `*api.RiskResponse`, matching every other operation's
response envelope. Change `result.AsLoginResponse()` to
`result.JSON201.AsLoginResponse()` and similarly move other assessment access
through `JSON201`. Request models remain generated. Some newly named component
types replace anonymous nested schemas; regenerate and compile callers when
updating the specification.

Releases remain in the `v0.x.y` series. New features and breaking public API changes
use a minor version increment; backward-compatible fixes use a patch increment.
