# Changelog

## v0.0.1

Initial release of the unofficial Castle SDK for Go. Requires Go 1.27.3 or later.

- Generate models and HTTP calls for all 27 operations in the checked-in official OpenAPI specification.
- Provide a shared client with Basic authentication, context cancellation, injectable HTTP clients, configurable response size limits, and safe errors.
- Support JSON and CSV responses, bodyless successes, dataset metadata, and presigned download locations without following redirects with Castle credentials.
- Regenerate both the API client and public endpoint methods through `go generate ./...` using a pinned toolchain and an unchanged upstream snapshot.
- Verify operations and serialization with synthetic local HTTP fixtures, race detection, static analysis, and generation checks in CI.

The public API is experimental and may change in subsequent v0 releases.
Live Castle service verification has not been performed for this release.
