---
paths:
  - "api/**"
  - "oapi-codegen.yaml"
  - "tools/openapibundle/**"
  - "dto/**"
  - "test/e2e/**"
---

# OpenAPI rules (spec-first)

The spec under `api/` is the **API contract** and the single source of truth for request/response shapes. It is written **before** code (step 0 of `tdd.md`). It is split per resource so each feature touches only its own files, and bundled into one generated file that tools read.

```
api/openapi.yaml + api/paths/*.yaml + api/schemas/*.yaml   (hand-written source)
        │  make generate → go run ./tools/openapibundle
        ▼
api/openapi.gen.yaml   (GENERATED bundle, single file, never edit)
        ├──oapi-codegen (models only)──▶ dto/dto.gen.go   (GENERATED, never edit)
        └──kin-openapi──▶ test/e2e validates every response against it
```

## Layout

```
api/
├── openapi.yaml         root: openapi/info/tags, the paths table, components.schemas index. Short; no operations or schema bodies inline
├── paths/<resource>.yaml        one file per path item, e.g. users.yaml (/api/v1/users), users_id.yaml (/api/v1/users/{id}), healthz.yaml
├── schemas/<resource>.yaml      one file per resource: a map of SchemaName → schema (User, CreateUserRequest, UpdateUserRequest, UserResponse, UserListResponse)
├── schemas/common.yaml          shared: ErrorResponse, ErrorBody, HealthResponse
└── openapi.gen.yaml             GENERATED bundle
```

- Root `paths:` entries are `$ref: ./paths/<file>.yaml`. Root `components.schemas:` lists **every** schema by its name: `User: {$ref: ./schemas/user.yaml#/User}`.
- Path files reference schemas by file + name: `$ref: ../schemas/user.yaml#/UserResponse`. Schema files reference each other the same way (`./common.yaml#/ErrorBody`, or `#/User` within the same file).
- Adding a feature = new `paths/*.yaml` + `schemas/<resource>.yaml`, plus one line per path and per schema in the root.
- Path parameters (e.g. `id`) are written inline in the path file. The bundler accepts external refs to **schemas only**; refs to parameters, responses, request bodies or examples in other files fail the bundle. Every top-level key in `schemas/*.yaml` must be listed in the root, or the bundle fails.

## Bundling (`tools/openapibundle`)

- A small Go program (kin-openapi loader with external refs allowed) that reads `api/openapi.yaml` and writes `api/openapi.gen.yaml`. The output has no external refs, has components named exactly as the root `components.schemas` keys, and refs written as `#/components/schemas/<Name>`. It validates the result (`doc.Validate`) and fails on error.
- `openapi.gen.yaml` is committed (like `repository/sqlcgen`), so readers and diffs see the full contract. `make check` fails when it is stale (see Generating).

## Writing the spec

- OpenAPI **3.0.3** (what oapi-codegen supports fully).
- Every operation has: `operationId` (camelCase, e.g. `createUser`), `tags`, every success and error response with its status, and a `$ref` to a schema for every body.
- **Schemas are named exactly as the Go dto types** they generate: `CreateUserRequest`, `UpdateUserRequest`, `User`, `UserResponse` (`{data: User}`), `UserListResponse` (`{data: [User]}`), `ErrorResponse` (`{error: ErrorBody}`), `ErrorBody`.
- Property names are snake_case (see `handler-dto.md`).
- Response schemas set `additionalProperties: false`, so the contract test fails on any undocumented field (e.g. a leaked `password_hash`).
- **No inline objects.** Every nested object is its own named schema referenced by `$ref`. Inline objects generate anonymous structs.
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

- oapi-codegen is pinned in `go.mod` as a Go tool (`go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0`).
- `make generate` runs, in order: `go run ./tools/openapibundle` → `go tool oapi-codegen -config oapi-codegen.yaml api/openapi.gen.yaml` → `sqlc generate`.
- `make check` includes `make check-generated`: it regenerates the bundle and dto into `tmp/_gen/` (gitignored; the `_` prefix keeps it out of `./...`), diffs them against the **working-tree** files, and runs `sqlc diff`. It writes no project files. Locally it proves the working tree is consistent; only a clean checkout (CI) proves the committed files are.
- Only models are generated. Server/router code is ours (Fiber v3 handlers + `router/router.go`).
- `dto/` contains only `dto.gen.go` plus, if needed, small hand-written files for things the spec can't express. Hand-written request/response structs that duplicate a schema are not allowed.
- The post-edit hook blocks hand edits to `api/openapi.gen.yaml` and `dto/*.gen.go`.

## Contract testing

- `test/e2e` loads `api/openapi.gen.yaml` with kin-openapi (`openapi3` + `openapi3filter` + `routers/gorillamux`) once in `TestMain`.
- The e2e HTTP helper validates **every response** against the matched operation (status code declared, body matches schema). An undeclared status or an undocumented field fails the test.
- Requests are not validated by the helper, because scenarios deliberately send invalid requests.
- A route that the spec doesn't declare fails the test (the router lookup fails), so every endpoint must be in the spec before its scenario runs.
