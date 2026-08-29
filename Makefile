# Collegiate Wrestling Rankings Board — task runner.
#
# Go commands run inside pipeline/ (the module root). DB is a Postgres
# connection string; override with: make migrate DB=postgres://...
# Defaults to $DATABASE_URL (see .env.example) when unset.

.PHONY: migrate seed test scrape resolve roster

migrate:
	cd pipeline && go run ./cmd/migrate -db="$(DB)" -dir=../db/migrations

seed:
	cd pipeline && go run ./cmd/seed -db="$(DB)" -dir=../db/seed

test:
	cd pipeline && go test ./...

# Ingest a FloWrestling season container. Defaults to the committed offline
# fixture so this runs with no network; pass URL=... for a live page and
# SEASON=2026 to override the inferred season.
scrape:
	cd pipeline && go run ./cmd/scrape -db="$(DB)" $(if $(URL),-url=$(URL),) $(if $(SEASON),-season=$(SEASON),)

# Second pass: reconcile raw name+school strings to canonical wrestlers.
# Pass ROSTER_SEASON=2027 to also resolve that season's WrestleStat roster
# entries (only meaningful after `make roster` has ingested them).
resolve:
	cd pipeline && go run ./cmd/resolve -db="$(DB)" $(if $(ROSTER_SEASON),-roster-season=$(ROSTER_SEASON),)

# Pull every D1 team's current roster from WrestleStat (docs/sources/wrestlestat.md).
# SEASON is required (WrestleStat's own ending-year convention, e.g. 2027 for
# the 2026-27 season) — there's no title to infer it from, unlike scrape.
roster:
	cd pipeline && go run ./cmd/roster -db="$(DB)" -season=$(SEASON)
