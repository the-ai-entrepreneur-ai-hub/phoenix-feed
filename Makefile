# phoenix-feed dev tasks. Plain Make. Works on git-bash, WSL, Linux, macOS.
# On Windows PowerShell, use scripts/dev.ps1 (TODO) or invoke targets via Make.

GO          := go
DOCKER      := docker
COMPOSE     := docker compose -f docker-compose.dev.yml

CMDS        := ingester api canary janitor
BIN_DIR     := bin

.PHONY: help
help:
	@echo "phoenix-feed targets:"
	@echo "  make tidy        - go mod tidy"
	@echo "  make build       - build all binaries into ./bin"
	@echo "  make test        - run all tests"
	@echo "  make lint        - go vet ./..."
	@echo "  make db-up       - start local Postgres+PostGIS via docker compose"
	@echo "  make db-down     - stop local DB"
	@echo "  make db-init     - apply db/schema.sql to local DB"
	@echo "  make db-migrate  - apply db/migrations/*.sql to local DB"
	@echo "  make ingester    - run cmd/ingester against local DB"
	@echo "  make api         - run cmd/api against local DB"
	@echo "  make pdf         - rebuild all design PDFs"
	@echo "  make smoke       - run local end-to-end smoke test"

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	@for cmd in $(CMDS); do \
		echo "build $$cmd"; \
		$(GO) build -o $(BIN_DIR)/$$cmd ./cmd/$$cmd || exit 1; \
	done

.PHONY: test
test:
	$(GO) test ./...

.PHONY: lint
lint:
	$(GO) vet ./...

.PHONY: db-up
db-up:
	$(COMPOSE) up -d db

.PHONY: db-down
db-down:
	$(COMPOSE) down

.PHONY: db-init
db-init:
	$(COMPOSE) exec -T db psql -U phoenix -d phoenix_feed < db/schema.sql

.PHONY: db-migrate
db-migrate:
	@for f in db/migrations/*.sql; do \
		if [ -f "$$f" ]; then \
			echo "apply $$f"; \
			$(COMPOSE) exec -T db psql -U phoenix -d phoenix_feed < "$$f" || exit 1; \
		fi; \
	done

.PHONY: ingester
ingester:
	DATABASE_URL=postgres://phoenix:phoenix@localhost:5432/phoenix_feed?sslmode=disable \
	$(GO) run ./cmd/ingester

.PHONY: api
api:
	DATABASE_URL=postgres://phoenix:phoenix@localhost:5432/phoenix_feed?sslmode=disable \
	$(GO) run ./cmd/api

.PHONY: pdf
pdf:
	python pdf-build/build.py
	python pdf-build/build_template.py

.PHONY: smoke
smoke:
	bash scripts/smoke.sh
