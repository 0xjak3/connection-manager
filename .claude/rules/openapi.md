---
paths:
  - "openapi.yaml"
  - "oapi-codegen.yaml"
  - "dto/**"
  - "test/e2e/**"
---

# OpenAPI rules (spec-first)

`openapi.yaml` at the project root is the **API contract** and the single source of truth for request/response shapes. It is written **before** code (the first part of the Red step in `tdd.md`).

```
openapi.yaml ──oapi-codegen (models only)──▶ dto/dto.gen.go   (GENERATED, never edit)
     │
     └──kin-openapi──▶ test/e2e validates every response against the spec
```

## Writing the spec

- OpenAPI **3.0.3** (what oapi-codegen supports fully).
- Every operation has: `operationId` (camelCase, e.g. `createUser`), `tags`, every success and error response with its status, and a `$ref` to a schema for every body.
- **Schemas are named exactly as the Go dto types** they generate: `CreateUserRequest`, `UpdateUserRequest`, `User`, `UserResponse` (`{data: User}`), `UserListResponse` (`{data: [User]}`), `ErrorResponse` (`{error: ErrorBody}`), `ErrorBody`.
- Property names are snake_case (see `handler-dto.md`).
- Response schemas set `additionalProperties: false`, so the contract test fails on any undocumented field (e.g. a leaked `password_hash`).
- **No inline objects.** Every nested object is its own schema referenced by `$ref`. Inline objects generate anonymous structs.
- `ErrorBody.code` is an enum of every error code in the error-handler table (`handler-dto.md`). Every error response in an operation lists the codes it can return in its `description`.
- Validation is written **twice, kept identical**: as schema constraints (`minLength`, `maxLength`, `required`, `minProperties`) for the contract/docs, and as `x-oapi-codegen-extra-tags: {validate: "..."}` for the go-playground tags the handler enforces. A reviewer rejects any mismatch.
- **Never** use `format: email` on request properties: oapi-codegen turns it into a type that validates during JSON decoding, which bypasses our validator (and rejects padded emails before trimming). Describe the format in `description` and enforce it with the validate tag. `format: uuid` and `format: date-time` are fine.
- Optional request fields (PATCH) are not `required` → generated as pointers with `omitempty`. Add `omitnil` in the validate tag.

## Generating

`oapi-codegen.yaml`:

```yaml
package: dto
output: dto/dto.gen.go
generate:
  models: true
output-options:
  skip-prune: true
  name-normalizer: ToCamelCaseWithInitialisms
compatibility:
  always-prefix-enum-values: true
```

- The tool is pinned in `go.mod` as a Go tool (`go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0`) and run by `make generate` (`go tool oapi-codegen -config oapi-codegen.yaml openapi.yaml`, together with `sqlc generate`).
- Only models are generated. Server/router code is ours (Fiber v3 handlers + `router/router.go`).
- `dto/` contains only `dto.gen.go` plus, if needed, small hand-written files for things the spec can't express. Hand-written request/response structs that duplicate a schema are not allowed.
- The post-edit hook blocks hand edits to `dto/*.gen.go`.

## Contract testing

- `test/e2e` loads `openapi.yaml` with kin-openapi (`github.com/getkin/kin-openapi/openapi3` + `openapi3filter` + `routers/gorillamux`) once in `TestMain`.
- The e2e HTTP helper validates **every response** against the matched operation (status code declared, body matches schema). An undeclared status or an undocumented field fails the test.
- Requests are not validated by the helper, because scenarios deliberately send invalid requests.
- A route that the spec doesn't declare fails the test (the router lookup fails), so every endpoint must be in the spec before its scenario runs.
