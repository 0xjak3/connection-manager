DATABASE_URL      ?= postgres://cm:cm@localhost:5440/connection_manager?sslmode=disable
TEST_DATABASE_URL ?= postgres://cm:cm@localhost:5441/connection_manager_test?sslmode=disable
MIGRATIONS        := db/migrations

.PHONY: generate db-up db-down migrate-up migrate-down test test-integration run lint-layers check test-e2e

generate: ## regenerate sqlc code
	sqlc generate

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

check: lint-layers ## full Definition of Done check (no DB)
	go vet ./...
	go build ./...
	go test ./...
	sqlc vet

test-e2e: ## e2e tests: throwaway Postgres container + real binary, HTTP only (needs Docker)
	go test -tags=e2e -count=1 ./test/e2e/...
