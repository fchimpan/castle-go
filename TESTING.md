# Testing the SDK

Use the principles in Vladimir Khorikov's *Unit Testing: Principles, Practices,
and Patterns* to balance regression protection, resistance to refactoring,
fast feedback, and maintainability.

## Test observable contracts

The root tests import `castle` as a consumer through `package castle_test`.
Model tests use `package api_test`. Assert returned assessments, error types,
HTTP status, cancellation, and serialized JSON. Avoid private client fields,
helper call order, and exact diagnostic wording.

Compare JSON objects by their decoded values. Object key order and whitespace
are not contracts. Header array order, omitted fields, explicit nulls, false,
and zero are contracts. The generated union's public concrete response type
is also observable to callers using a type switch.

Keep each scenario focused on one outcome. Multiple assertions are appropriate
when they describe that outcome, such as an API failure returning both a nil
assessment and an error with the correct status. Use explicit expected values
instead of deriving expectations from private production constants.

## Keep real collaborators inside the SDK

Use the actual generated models and client. Do not mock their internal calls.
Local `httptest` servers cover the HTTP boundary, including authentication,
redirect rejection, and cancellation during a request or response read.
These are integration tests; pure JSON serialization tests are unit tests.

Injected HTTP transports supply otherwise difficult failures and response bodies.
Request counts matter for the no-retry contract. Requests reaching a redirect
destination matter for credential protection. Resource cleanup matters because
the caller supplies the transport. Avoid assertions about unrelated interactions
with test doubles.

Each parallel case owns its server, HTTP transport, and mutable state. Use
`server.Client()` for local fixtures to isolate connection pools. Coordinate cancellation
with channels rather than sleeps, and provide a cleanup path if an assertion
fails. Use synthetic data and no Castle credentials. Check all event variants in
model tests without repeating the same authentication test for every variant.

The schema preparation tests exercise the pure transformation and checksum
boundary without starting a Go subprocess for every case. CI separately verifies
the complete `go generate ./...` workflow.

`testdata/operations.json` contains synthetic fixtures based on the checked-in
official specification, including recursive query filters, optional false and
zero values, explicit nulls, and array responses. `TestEveryDocumentedOperation`
requires a fixture for every OpenAPI operation, including HEAD and deprecated
routes. It calls the public method, observes the real HTTP request, and checks
the returned typed payload. Reflection lets one fixture runner call methods with
different public signatures without duplicating the assertions. It does not
inspect private client state. Focused tests additionally check CSV, typed response
headers, download redirects, and errors before and after transport.

## Run checks

```sh
go test -race -count=1 ./...
go vet ./...
go generate ./...
git diff --exit-code
```

CI runs generation verification, tests with race detection, and static analysis
in separate jobs. Tests use `-count=1` to bypass cached results.
Do not use a coverage percentage as the sole measure
of test quality.

The author's explanations of [observable behavior](https://khorikov.org/posts/2021-01-04-unit-testing-vs-bdd/)
and [when to use mocks](https://enterprisecraftsmanship.com/posts/when-to-mock/)
provide the rationale for these choices.
