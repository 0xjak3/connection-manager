DATABASE_URL      ?= postgres://cm:cm@localhost:5440/connection_manager?sslmode=disable
TEST_DATABASE_URL ?= postgres://cm:cm@localhost:5441/connection_manager_test?sslmode=disable
MIGRATIONS        := db/migrations

.PHONY: generate check-generated db-up db-down migrate-up migrate-down test test-integration run lint-layers check test-e2e

generate: ## bundle api/ → api/openapi.gen.yaml, generate dto/dto.gen.go, then sqlc code
	go run ./tools/openapibundle
	go tool oapi-codegen -config oapi-codegen.yaml api/openapi.gen.yaml
	sqlc generate

# Regenerates into tmp/_gen (ignored by ./... because of the _ prefix) and diffs against the working tree; touches no project files.
check-generated: ## fail if generated code is stale
	@rm -rf tmp/_gen && mkdir -p tmp/_gen
	@go run ./tools/openapibundle -out tmp/_gen/openapi.gen.yaml
	@sed 's#^output: .*#output: tmp/_gen/dto.gen.go#' oapi-codegen.yaml > tmp/_gen/oapi-codegen.yaml
	@go tool oapi-codegen -config tmp/_gen/oapi-codegen.yaml tmp/_gen/openapi.gen.yaml
	@diff -u api/openapi.gen.yaml tmp/_gen/openapi.gen.yaml && diff -u dto/dto.gen.go tmp/_gen/dto.gen.go && sqlc diff || \
		{ echo "generated code is stale: run 'make generate'" >&2; exit 1; }
	@echo "generated code: up to date"

db-up: ## start dev + test Postgres and wait until healthy
	docker compose up -d --wait db db-test

db-down: ## stop the project's containers (keeps the dev volume)
	docker compose down

# Uses the migrate CLI if installed, otherwise the migrate/migrate docker image.
migrate-up: ## apply all migrations to the dev DB
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" up; \
	else \
		docker run --rm --add-host=host.docker.internal:host-gateway \
			-v "$(CURDIR)/$(MIGRATIONS):/migrations" migrate/migrate \
			-path=/migrations -database "$(subst localhost,host.docker.internal,$(DATABASE_URL))" up; \
	fi

migrate-down: ## roll back the last migration on the dev DB
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" down 1; \
	else \
		docker run --rm --add-host=host.docker.internal:host-gateway \
			-v "$(CURDIR)/$(MIGRATIONS):/migrations" migrate/migrate \
			-path=/migrations -database "$(subst localhost,host.docker.internal,$(DATABASE_URL))" down 1; \
	fi

test: ## unit tests, no DB needed
	go test ./...

test-integration: ## repository integration tests against the test DB
	docker compose up -d --wait db-test
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration -count=1 ./repository/...

run: ## run the API against the dev DB
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

lint-layers: ## check layer import boundaries on all .go files
	./.claude/hooks/check-all.sh

check: lint-layers check-generated ## full Definition of Done check (no DB)
	go vet ./...
	go build ./...
	go test ./...
	sqlc vet

test-e2e: ## e2e tests: throwaway Postgres container + real binary, HTTP only (needs Docker)
	go test -tags=e2e -count=1 ./test/e2e/...
