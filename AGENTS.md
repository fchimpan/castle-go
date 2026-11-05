# Contributor guidance

## Purpose and completion criteria

This repository implements an open-source Go SDK for Castle, a risk assessment SaaS.

The SDK is complete for a checked-in specification when:

1. Reproducible generation produces models and endpoint calls for every operation in Castle's official OpenAPI definition, including deprecated operations and HEAD requests.
2. A shared client exposes those operations with consistent configuration, authentication, context handling, and safe errors.
3. Local tests verify endpoint coverage and observable request and response behavior, and regeneration, tests, and static checks pass.

Generate all API families by default and keep endpoint coverage driven by the specification. Add handwritten behavior only where generation does not satisfy the shared client contract. Clearly distinguish specification coverage, locally verified behavior, and live-service verification.

## API specification

- Treat the [official API reference](https://reference.castle.io/) as the source of truth for every API family.
- The reference uses a machine-readable [OpenAPI definition](https://reference.castle.io/v1/api.json). Use this definition as the code generation input rather than transcribing types from rendered documentation.
- Verify endpoint paths, authentication, request and response fields, optionality, and error behavior against the documentation before implementing them. Do not guess missing details.
- When implementation details are unclear, consult the official [Java SDK](https://github.com/castle/castle-java) and [Node SDK](https://github.com/castle/castle-node). Prefer the API reference when they differ; make unresolved discrepancies explicit.
- Adapt SDK behavior to idiomatic Go rather than copying another language's class structure.

## Code generation workflow

1. Inspect the working tree and installed Go version. Use module `github.com/fchimpan/castle-go` with a minimum of Go 1.27.3.
2. Download the official OpenAPI definition and check it into the repository, for example as `api/openapi.json`. Record its source URL, retrieval date, and checksum. Keep the upstream snapshot unchanged.
3. Use a pinned release of [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) with the actual Castle schema. Generate the models and HTTP client for all operations before designing handwritten equivalents. Inspect the resulting API and compile it. Do not assume generator support from its feature list alone.
4. Check the generated treatment of `oneOf`, `allOf`, discriminators, nullable fields, required fields, enums, and `additionalProperties`. Exercise representative request and response JSON. Choose the public SDK API after inspecting these results.
5. If the schema needs a correction or a generator needs an adaptation, record the reason and supporting source. Apply the change through a checked-in overlay, configuration, or deterministic preprocessing step. Keep the original snapshot intact. Compare another generator when a concrete limitation warrants it. Do not silently fall back to handwritten copies of the schema.
6. Pin the generator version and commit its configuration. Provide `go generate ./...` as the regeneration entry point. Regeneration must use the checked-in specification and pinned tool, without fetching the latest specification or tool version implicitly.
7. Commit generated Go files with their generated-file headers. Keep generated and handwritten code distinguishable. Never edit generated output directly; change the input, overlay, or configuration and regenerate.
8. Add only the handwritten SDK behavior that generation does not provide. Document how to update the specification and regenerate. Verify that a second generation produces no changes, and keep the CI check for stale generated output.

Prefer generated request models, response models, and endpoint calls over handwritten duplicates. Keep configuration, authentication setup, safe error handling, and necessary usability adapters in a small handwritten layer. Reuse generated transport hooks where they satisfy the SDK contract. Avoid creating a second complete model hierarchy just to rename generated types. Accept generator runtime dependencies when needed, but keep them deliberate and minimal.

Evaluate these details against the downloaded schema before accepting generated code. The following observations were verified on 2026-11-03 and must be rechecked when the schema changes:

- The base URL is `https://api.castle.io/v1`. Risk uses `POST /risk` and returns `201` on success.
- Basic authentication uses an empty username and the API secret as the password.
- Risk has eight event variants. The shared request marks status required, while some event variants specify defaults. Custom status permits null.
- Explicit validation skip flags allow omission of the request token or context. Skipping request token validation prevents risk score calculation. Missing scores must remain distinguishable from zero scores.
- Request `BaseRequest.properties` has an additional object level compared with response `Properties`. Verify this against the official schema and SDK examples rather than flattening it for convenience.
- Context headers are ordered pairs, with string, boolean, or integer scalar values in the schema. Check how the generator represents these tuples and unions.
- Changesets include known fields, custom keys, and alternative value shapes. Authentication-method changeset prose permits null endpoints although the referenced enum does not declare nullability. Record the chosen interpretation and verify omission versus explicit null.
- The documented Risk failures are `401`, `403`, `422`, `429`, and `500`. Only `422` has a body schema in the inspected endpoint definition. Preserve HTTP status even for empty or malformed error bodies.

## Go implementation and validation

- Use Go 1.27.3 as the minimum version. Keep `go.mod`, documentation, and CI aligned. Do not lower the version based on an unverified compatibility assumption.
- Prefer standard language and library features. Use `new(value)` for pointers to values instead of adding a `Ptr` helper. Run `go fix ./...` when modernizing handwritten code, and inspect its changes. Changes to generated code must come through the generation workflow.
- Accept `context.Context` for API calls and support an injectable HTTP client for configuration and testing.
- Keep shared transport and error handling in the client. Preserve context cancellation and deadline detection through `errors.Is`.
- Return actionable errors without exposing API secrets or sensitive request data.
- Prevent credentials from being forwarded through redirects. Do not silently retry assessment requests or turn an API failure into an allow decision.
- Derive operation success statuses from the specification, including bodyless responses and documented redirects. Return a download's presigned Location without automatically following it with Castle credentials.
- Prefer the latest stable dependencies that compile and pass the SDK checks. Pin reproducible versions and document compatibility exceptions for older versions.
- Test request serialization, authentication, response decoding, API failures, and context cancellation with local HTTP fixtures such as `httptest`.
- Follow *Unit Testing: Principles, Practices, and Patterns*: test observable behavior through public APIs, use real internal collaborators, and keep external-boundary test doubles focused on meaningful outcomes. Keep CI generation, tests with race detection, and vet in separate jobs.
- Test generated JSON behavior as well as handwritten code. Cover optional false and zero values, null versus omission where the API distinguishes them, event variants, expanded responses, and unknown additional response fields. Expected fixtures must come from verified API behavior, not solely from the generated types being tested.
- Default tests must not require live credentials or access to Castle services. Use synthetic fixture data.
- Use named, table-driven subtests with `t.Parallel()` when cases are independent. Format structured table entries across multiple lines with field names. Do not compress nested request fixtures into one-line literals.
- Give parallel cases their own mutable state and HTTP fixtures. A parent's deferred server closure runs before its parallel subtests. Use case-owned fixtures or `t.Cleanup`, and ensure assertions run after the requests they verify.
- Once the Go module exists, format handwritten Go changes with `gofmt` and run `go test ./...` and `go vet ./...` before completing implementation changes. Run `go test -race ./...` when adding or changing parallel tests. Verify regeneration leaves generated files unchanged. Report checks that could not run.

## Language

Use English for all repository contributions, including identifiers, comments, documentation, examples, test descriptions, error messages, commit messages, and pull request text. Conversation with the maintainer may follow their language preference.

## Versioning

- Keep releases in the `v0.x.y` series until a stable major release is explicitly planned.
- Use minor version increments for new features or breaking public API changes, and patch increments for backward-compatible fixes.
- Document public API changes and migration steps when compatibility changes, even during 0.x development.
