# Collegiate Wrestling Rankings Board — task runner.
#
# Go commands run inside pipeline/ (the module root). The SQLite database lives
# at the repo root by default; override with: make migrate DB=path/to.db

DB ?= rankings.db

.PHONY: migrate seed test scrape

migrate:
	cd pipeline && go run ./cmd/migrate -db=../$(DB) -dir=../db/migrations

seed:
	cd pipeline && go run ./cmd/seed -db=../$(DB) -dir=../db/seed

test:
	cd pipeline && go test ./...

scrape:
	@echo "not implemented — pending Flo recon"
