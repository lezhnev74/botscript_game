GOLANGCI_LINT ?= golangci-lint
COVER_PROFILE := coverage.out

.PHONY: e2e test cover crap lint check determinism gen gen-check run fixtures fixtures-check web-deps web-test web-typecheck web-lint web-build docker docker-run

test:
	cd engine && go test ./...

# cover is the single test run (with coverage across all packages).
cover:
	cd engine && go test -coverpkg=./... -coverprofile=$(COVER_PROFILE) ./...

# crap runs after lint even under make -j (order-only prerequisite is not enough; real dependency).
crap: lint cover
	cd engine && go run ./tools/crap -root . -profile $(COVER_PROFILE) -max 8

lint:
	cd engine && $(GOLANGCI_LINT) run ./...

# run rebuilds the web app, then starts the server; type admin commands in this terminal (stdin): list, kick <id|name>, reset-board, min-bots <n>, timer <minutes>, start, help (PLAN.md section 4).
# Set ARENA_ADMIN_PASSWORD (env var or arena/.env) to enable the web admin at /admin, ARENA_BASE_URL (e.g. https://arena.example.com) to put the server address in the AI rulebook.
# Paths are relative to arena/ (override: make run ADDR=:9000 BEAT=300).
ADDR ?= :8080
BEAT ?= 1000
run: web-build
	cd engine && go run ./cmd/arena -addr $(ADDR) -beat $(BEAT) -web ../web/dist -data ../data -env ../.env

# gen turns the Go wire types (engine/adapters/protocol) into web/src/protocol.
gen:
	cd engine && go tool tygo generate
	cd web && node scripts/gen-docs.mjs

# gen-check fails when the committed TypeScript protocol is stale.
gen-check:
	@set -e; d=$$(mktemp -d); trap 'rm -rf "$$d"' EXIT; \
	sed "s|output_path:.*|output_path: \"$$d/protocol.ts\"|" engine/tygo.yaml > "$$d"/tygo.yaml; \
	(cd engine && go tool tygo generate --config "$$d"/tygo.yaml >/dev/null); \
	diff "$$d"/protocol.ts web/src/protocol/protocol.ts || { echo "web/src/protocol is stale: run make gen and commit"; exit 1; }; \
	(cd web && node scripts/gen-docs.mjs "$$d"/generated); \
	diff -r "$$d"/generated web/src/generated || { echo "web/src/generated is stale: run make gen and commit"; exit 1; }; \
	echo "gen-check: web/src/protocol and web/src/generated are up to date"

# fixtures exports scenarios and long records as the WebSocket message
# sequences the UI develops and tests against (web/fixtures/<name>.json).
SCN := testdata/scenarios
LONG := testdata/long
FIXTURE_INPUTS := $(SCN)/move_free_cell.scn $(SCN)/scan_enemy_level3.scn $(SCN)/bullet_hits_stationary_target.scn $(SCN)/bullet_travels_three_cells_per_beat.scn \
	$(SCN)/respawn_after_five_beats.scn $(SCN)/coin_pickup_gives_bonus.scn \
	$(SCN)/program_hot_swap_keeps_memory_hp_and_reload.scn $(SCN)/program_pin_here_returns_to_the_pinned_cell.scn $(LONG)/duel_5x5.record.json $(LONG)/four_9x9.record.json
FIXTURE_DIR ?= $(CURDIR)/web/fixtures
fixtures:
	@set -e; mkdir -p "$(FIXTURE_DIR)"; cd engine; for f in $(FIXTURE_INPUTS); do go run ./cmd/replay -export "$(FIXTURE_DIR)" $$f >/dev/null; done

# fixtures-check fails when the committed fixtures are stale.
fixtures-check:
	@set -e; d=$$(mktemp -d); trap 'rm -rf "$$d"' EXIT; \
	$(MAKE) --no-print-directory fixtures FIXTURE_DIR="$$d"; \
	diff -r "$$d" web/fixtures || { echo "web/fixtures is stale: run make fixtures and commit"; exit 1; }; \
	echo "fixtures-check: web/fixtures is up to date"

# web-deps fails clearly when `npm ci` was not run (make check never installs).
web-deps:
	@test -d web/node_modules || { echo "web/node_modules is missing: run (cd web && npm ci)"; exit 1; }

web-test: web-deps
	cd web && npm test

web-typecheck: web-deps
	cd web && npm run typecheck

web-lint: web-deps
	cd web && npm run lint

web-build: web-deps
	cd web && npm run build

# e2e is the Playwright smoke test (builds the web app, starts cmd/arena on a random port); not part of check.
e2e: web-build
	cd web && node e2e/smoke.mjs

# docker builds the image; docker-run serves it on host port $(PORT) with a named data volume (not part of check).
IMAGE ?= arena
PORT ?= 8080
docker:
	docker build -t $(IMAGE) .

docker-run:
	docker run --rm --name $(IMAGE) -p $(PORT):8080 -v $(IMAGE)-data:/data $(if $(wildcard .env),--env-file .env) -e ARENA_ADMIN_PASSWORD -e ARENA_BASE_URL $(IMAGE)

check: crap determinism gen-check fixtures-check web-typecheck web-lint web-test

# determinism is proof 3 (scheduler independence): the scenario + random-match
# hashes must be byte-identical under GOMAXPROCS=1, GOMAXPROCS=8 and -race.
# It also runs the seed corpus of FuzzMatch. Part of `make check`.
determinism:
	@set -e; d=$$(mktemp -d); trap 'rm -rf "$$d"' EXIT; \
	cd engine; \
	GOMAXPROCS=1 ARENA_HASH_OUT="$$d"/p1.txt go test -count=1 -run 'TestDeterminism' ./scenario; \
	GOMAXPROCS=8 ARENA_HASH_OUT="$$d"/p8.txt go test -count=1 -run 'TestDeterminism' ./scenario; \
	GOMAXPROCS=8 ARENA_HASH_OUT="$$d"/race.txt go test -race -count=1 -run 'TestDeterminism' ./scenario; \
	GOMAXPROCS=1 go test -count=1 -run 'FuzzMatch|Replay' ./app; \
	go test -race -count=1 -run 'FuzzMatch|Replay' ./app; \
	cmp "$$d"/p1.txt "$$d"/p8.txt; cmp "$$d"/p1.txt "$$d"/race.txt; \
	echo "determinism: $$(wc -l < "$$d"/p1.txt) hashes identical under GOMAXPROCS=1, 8 and -race"
