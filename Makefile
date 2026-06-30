# Collegiate Wrestling Rankings Board — task runner.
#
# Go commands run inside pipeline/ (the module root). The SQLite database lives
# at the repo root by default; override with: make migrate DB=path/to.db

DB ?= rankings.db

.PHONY: migrate seed test scrape resolve

migrate:
	cd pipeline && go run ./cmd/migrate -db=../$(DB) -dir=../db/migrations

seed:
	cd pipeline && go run ./cmd/seed -db=../$(DB) -dir=../db/seed

test:
	cd pipeline && go test ./...

# Ingest a FloWrestling season container. Defaults to the committed offline
# fixture so this runs with no network; pass URL=... for a live page and
# SEASON=2026 to override the inferred season.
scrape:
	cd pipeline && go run ./cmd/scrape -db=../$(DB) $(if $(URL),-url=$(URL),) $(if $(SEASON),-season=$(SEASON),)

# Second pass: reconcile raw name+school strings to canonical wrestlers.
resolve:
	cd pipeline && go run ./cmd/resolve -db=../$(DB)
