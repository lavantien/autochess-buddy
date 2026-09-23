# autochess companion mvp implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** implement the complete localhost autochess companion mvp from the two reviewed specs: manual codex + match/lineup entry, duckdb analytics dashboard, server-rendered templ + htmx 4 ui.

**Architecture:** one go binary, sqlite source of truth (journal TRUNCATE, committed to git), duckdb attached read-only over the same file behind a mutex shared with the write path, templ partials swapped by htmx, no client js beyond htmx. Layers bottom-up: domain -> store/sqlite -> service -> httpapi -> ui, analytics as its own layer behind a service interface.

**Tech stack:** go 1.27, templ v0.3.1020, htmx 4.0.0 vendored, mattn/go-sqlite3, pressly/goose/v3 (provider api), github.com/duckdb/duckdb-go/v2 (@latest, record resolved tag), pgregory.net/rapid, playwright-go. All pins verified 2026-09-23 (today) except duckdb-go which resolves at scaffold.

**Spec:** `readme.md` (architecture, schema, route table, dag) and `frontend-design.md` (tokens, screens, swap contract, copy, a11y). Both travel with this plan; executors read the relevant spec section before each task. Verbatim inventories (v1 schema at readme:104-182, route table at readme:213-236, stable ids and swap table at frontend-design:259-293, copy at frontend-design:318-363, tokens at frontend-design:28-78) are normative and are not restated here.

## Global Constraints

- CGO_ENABLED=1 on every go invocation; pwsh 7 or git bash on windows; gcc 15.2 (mingw) already on PATH.
- Max 400 SLOC per hand-written file; `*_templ.go` are generated, never hand-edited, always regenerated before commit.
- TDD: failing test first, minimal pass, refactor. Tests + implementation land in the same commit.
- Conventional Commits only: feat, fix, docs, refactor, test, chore.
- No AI attribution anywhere: no Co-Authored-By, no Generated with, in any commit message or file.
- No docker, no json api, no client framework, no build step beyond templ generate.
- Each layer imports only the layer directly below (readme:96); domain is stdlib-only.
- No interface ceremony beyond the spec-mandated `analytics.Service` (readme:243).
- Baseline before chain: stage 0 proves `go build ./...` + empty test suite green before any feature work.
- Subagent fan-out at execution: max 4 concurrent agents per the dag; slots recycled as tasks finish; each task commits atomically.

## Review Focus

Input classes the specs imply but no happy path exercises; each is pinned by a named test in its owning task.

1. Finalize a pro match with 7 lineups or duplicate placements -> editor rerenders with the banner naming the missing lineups or the duplicate (Task A.2 `TestValidateFinalize_*`, Task E.2 `TestFinalize_InvalidRendersBannerNamingProblem`).
2. Free-text hero name absent from the codex (datalist is a suggestion) -> 422 heroform rerender with values preserved and "no hero named x in the codex. add it first." (Task E.2 `TestAddSlot_UnknownHeroRendersSpecCopy`).
3. Deleting a codex entity referenced by match history -> sqlite FK violation maps to `domain.ErrInUse`, delete answers with the "existing matches keep their history" messaging (Task B.1 `TestDeleteHero_InUseMapsToErrInUse`, Task E.3 `TestDeleteHero_InUseRendersConflict`).
4. Non-hx request to any partial or mutation route (bookmark, no-js) -> full page or 303 redirect, never a bare fragment (Task C.2 `TestIsHX`, Task E.2/E.4 non-hx handler tests).
5. Double submit of add lineup (enter key racing the button) -> `hx-disable` + `hx-sync` client side, placement unique constraint server side maps to `PlacementConflictError` with spec copy (Task E.2 `TestAddLineup_PlacementConflictRendersSpecCopy`).
6. Duckdb staleness and lock fights -> analytics batch holds the shared write mutex, duckdb sees committed sqlite state (Task D.1 `TestEngine_BatchHoldsSharedMutex`, `TestWriteThenRead_DuckdbSeesCommittedSqlite`).

---

## Concurrency map (execution)

```
stage 0 (main session, sequential)
A: one agent (migrations + domain + seed)
then B, C, D in parallel (3 agents, under the 4 cap)
E: one agent (needs B + C done; D may still finish)
F: main session + 2 adversarial agents in parallel (needs D + E)
```

Stage A owns every cross-stage type (domain, store open/WithTx signatures, analytics view structs) so parallel stages never edit the same file. Stage C stays free of store/analytics imports (stub pages only) so it needs no seams.

## Recorded deviations from spec text (decided, do not re-litigate)

1. `matches.finalized_at INTEGER NOT NULL DEFAULT 0` added to migration 00001. The readme v1 schema omits it but its own sequence diagram says "one tx: mark finalized" (readme:74) and the matches list filters on state draft/final. Deriving state from lineup counts would auto-finalize 8-lineup drafts and make finalize a no-op. Analytics filters `finalized_at > 0`.
2. Wilson 95% lower bound is a pure Go function `WilsonLB(k, n)` applied to counts SQL returns, not SQL. Resolution of readme:206 ("all math stays in sql") vs readme:260 (property test "wilson lower bound monotone in n"): SQL keeps all aggregation, Go applies one deterministic scalar to assembled counts; a comment in wilson.go records this.
3. Swap table row "add lineup -> cards, pips, addform" vs the non-nesting rule (frontend-design:267): the add form lives inside the cards grid region, so its reset rides the cards re-render; responses emit cards + pips only.
4. `POST /matches` (sequence diagram) vs `POST /matches/new` (route table + swap table): use `/matches/new`.
5. `data/app.db` is committed migrated and empty; `task seed` is opt-in so fake fixture data never mixes with real manual entry.
6. duckdb-go "v2.5+" is a floor; tags encode the bundled duckdb version (e.g. v2.10505.0 = 1.5.5). Pin @latest once at scaffold, record the resolved tag in go.mod, never float after.

---

## Stage 0: scaffold (main session, sequential)

### Task 0.1: toolchain + module + vendored assets

**Files:** Create: `go.mod`, `Taskfile.yml`, `.gitignore`, `.golangci.yml`, `.githooks/pre-commit`, `internal/ui/static/htmx.min.js`, `internal/ui/static/fonts/{barlow-400,barlow-500,barlow-condensed-600,barlow-condensed-700}.woff2`, `docs/superpowers/plans/2026-09-23-autochess-mvp.md` (copy of this plan file).

- [ ] Verify baseline: `git remote -v` -> module path `github.com/lavantien/autochess-buddy`; `go mod init github.com/lavantien/autochess-buddy`.
- [ ] `go get` deps: `github.com/mattn/go-sqlite3`, `github.com/pressly/goose/v3`, `github.com/duckdb/duckdb-go/v2@latest` (record resolved tag), `github.com/pgregory.net/rapid`, `github.com/a-h/templ@v0.3.1020`; add `tool github.com/a-h/templ/cmd/templ` go directive; `github.com/playwright-community/playwright-go` as test dep.
- [ ] Vendor htmx: `npm pack htmx.org@4.0.0` (next dist-tag), extract `package/dist/htmx.min.js` (verify the path against the tarball listing) into `internal/ui/static/`.
- [ ] Vendor fonts: fetch the google fonts css2 urls for Barlow 400, Barlow 500, Barlow Condensed 600, Barlow Condensed 700 with a Chrome UA, download the latin-subset woff2 files into `internal/ui/static/fonts/` (OFL licensed).
- [ ] `.gitignore`: `data/*.db-journal`, `*.tgz`, `node_modules/`.
- [ ] `.githooks/pre-commit` (git runs it via its sh; wire once with `git config core.hooksPath .githooks`):

```sh
#!/bin/sh
# a 0-byte journal is the idle steady state of TRUNCATE mode; non-zero means an
# in-flight transaction and the db on disk is torn.
if [ -s data/app.db-journal ]; then
  echo "refusing to commit: data/app.db-journal is non-zero (transaction in flight). close the app or let it finish." >&2
  exit 1
fi
if git diff --cached --name-only | grep -q 'app\.db-journal$'; then
  echo "refusing to commit: data/app.db-journal is staged." >&2
  exit 1
fi
```

- [ ] `Taskfile.yml`: top-level `env: CGO_ENABLED: "1"`; targets `gen` (`go tool templ generate`), `fmt` (gofmt -l -w . + `go tool templ fmt .`), `lint` (golangci-lint run), `vet` (go vet ./...), `test` (go test ./...), `e2e` (go test -tags=e2e ./e2e/... -count=1), `playwright` (go run the playwright install helper, Task F.1), `seed` (go run ./cmd/autochess -seed), `serve`, `dev` (deps: templ watch + serve), `check` (deps: gen, fmt, lint, vet, test, e2e, then `git diff --exit-code -- internal/ui` so stale generated files cannot land).
- [ ] `.golangci.yml`: minimal (default linters + govet, errcheck, staticcheck, unused).
- [ ] Baseline: `CGO_ENABLED=1 go build ./...` and `go test ./...` green on the empty module.
- [ ] Commit: `chore: scaffold module, taskfile, hooks, vendored static assets`.

---

## Stage A: migrations + domain + seed fixture (one agent)

### Task A.1: sqlite open, migrations, WithTx

**Files:** Create: `internal/store/sqlite/open.go`, `internal/store/sqlite/migrations/00001_init.sql`.

**Interfaces (produced, consumed by B/C/E):**

```go
package sqlite
type Store struct {
    DB      *sql.DB
    WriteMu sync.Mutex // serializes every write tx against duckdb read batches
}
func Open(path string) (*Store, error) // opens pool with per-conn pragmas, runs migrations
func (s *Store) Close() error
func (s *Store) WithTx(ctx context.Context, fn func(*sql.Tx) error) error // locks WriteMu for the whole tx
```

- [ ] Write `00001_init.sql`: the readme v1 schema verbatim (readme:104-182) plus the `finalized_at` deviation on `matches`, with a goose `-- +goose Up` annotation.
- [ ] Failing test first: `TestOpen_MigratesFreshFile` (t.TempDir real file, never :memory:, per readme:199; assert `patches` etc. exist via sqlite_master), `TestOpen_SecondOpenIsNoop` (goose version table unchanged), `TestWithTx_RollsBackOnError` (insert then error, count unchanged), `TestWithTx_HoldsWriteMu` (WithTx body observes `TryLock` fails).
- [ ] Implement: DSN `path + "?_journal_mode=TRUNCATE&_busy_timeout=5000&_foreign_keys=on"` (mattn applies these per pool conn), `//go:embed migrations/*.sql` + `fs.Sub` + `goose.NewProvider(goose.DialectSQLite3, db, sub)` + `p.Up(ctx)` (provider api verified today; legacy globals avoided).
- [ ] Run tests green. Commit: `feat: v1 schema migration with goose provider bootstrap`.

### Task A.2: domain types, validation, sentinel errors

**Files:** Create: `internal/domain/codex.go`, `internal/domain/match.go`, `internal/domain/validate.go`, `internal/domain/util.go`; tests alongside.

**Interfaces (produced, consumed everywhere):** types `Patch, Race, Class, Tier, Hero, Item, Relic, Pro, Match, Lineup, Slot, AddLineupCmd`; `FieldError{Field, Msg}`, `ValidationError []FieldError`, `PlacementConflictError{Placement int}`; sentinels `ErrInUse, ErrSlotCap, ErrItemCap, ErrStarsRange, ErrCostRange, ErrLineageCount, ErrNotFound`; funcs `ValidateHero(Hero) error`, `ValidateFinalize(Match, []Lineup) error`, `ValidateAddSlot([]Slot) error`, `ValidateSlotStars(int) error`, `ValidateAddItem([]int64) error`, `NextSlotIndex([]Slot) int`, `FirstFreePlacement([]Lineup) int`, `ParsePlayedAt(string) (int64, error)`. Stdlib only (readme:190).

- [ ] Failing table tests: `TestValidateFinalize_MeNeedsExactlyOneLineup`, `TestValidateFinalize_ProNeedsEightDistinctPlacements`, `TestValidateFinalize_ProDuplicatePlacementsRejected`, `TestValidateAddSlot_RejectsThirteenthSlot`, `TestValidateAddItem_RejectsSeventhItem`, `TestValidateHero_CostRange`, `TestValidateHero_LineageOneToTwo`, `TestParsePlayedAt_RoundTrip` (datetime-local string -> unix -> back), `TestParsePlayedAt_RejectsGarbage`.
- [ ] rapid property tests: `TestNextSlotIndex_FillsLowestGap_PermutationInvariant` (readme:260), `TestFirstFreePlacement_SkipsUsed_PermutationInvariant`.
- [ ] Implement. All error copy comes from frontend-design:357-363 verbatim (errors live in domain so stores and handlers render identical text).
- [ ] Green. Commit: `feat: domain types, validation rules, error vocabulary`.

### Task A.3: wilson lower bound

**Files:** Create: `internal/domain/wilson.go` + test.

**Interfaces:** `func WilsonLB(k, n int) float64` (z = 1.96, 95% lower bound; n = 0 returns 0).

- [ ] Failing tests: `TestWilsonLB_ZeroN`, `TestWilsonLB_Extremes` (k=0, k=n), rapid `TestWilsonLB_NeverExceedsObservedRate`, rapid `TestWilsonLB_MonotoneInNAtFixedRate` (readme:260).
- [ ] Implement with the deviation-2 comment. Green. Commit: `feat: wilson 95 percent lower bound with property tests`.

### Task A.4: analytics view contracts

**Files:** Create: `internal/analytics/views.go` (pure types + interface, no engine import).

**Interfaces (consumed by D and E):**

```go
package analytics
type Filter struct{ PatchID int64; Source string } // zero values mean all
type HeroRow struct {
    Hero domain.Hero; Picks, Top4, LineupsInView int; AvgPlace float64; Finishes [8]int
    PickRate, Top4Rate, Floor, VsField float64 // assembled in D
}
type SynergyRow struct{ Kind string /*race|class*/; ID int64; Name string; TierCount, Lineups int; Lift float64; Finishes [8]int }
type ItemRow struct{ Item domain.Item; SlotsWith, LineupsInView int; Lift float64; Finishes [8]int }
type RelicRow struct{ Relic domain.Relic; Lineups, LineupsInView int; Lift float64; Finishes [8]int }
type PlaceRow struct{ Placement, N int; AvgNetworth float64 }
type Service interface {
    HeroPerformance(ctx context.Context, f Filter) ([]HeroRow, error)
    SynergyPerformance(ctx context.Context, f Filter) ([]SynergyRow, error)
    ItemPerformance(ctx context.Context, f Filter) ([]ItemRow, error)
    RelicPerformance(ctx context.Context, f Filter) ([]RelicRow, error)
    NetworthByPlacement(ctx context.Context, f Filter) ([]PlaceRow, error)
    LineupsInView(ctx context.Context, f Filter) (int, error)
}
```

- [ ] Compile-only task (types carry no logic); the interface is the readme:243 mandate. Commit: `feat: analytics view contracts and service interface`.

### Task A.5: shared seed fixture

**Files:** Create: `internal/seed/fixture.sql`, `internal/seed/seed.go`.

**Interfaces:** `func Load(db *sql.DB) error`, sentinel `ErrSeeded` (refuses a db that already has matches; single multi-statement Exec).

- [ ] Fixture content, commented so goldens are hand-derivable (placement multisets per hero written as sql comments): 2 patches (7.4, 7.5); 3 races + 3 classes with tier ladders (counts 2/4/6); 12 heroes across costs 1-5 including 2 dual-lineage pieces; 8 items with 2 recipes; 4 relics; 4 pros; 6 matches = 4 pro finalized (2 per patch, placements 1-8 distinct in each) + 1 me finalized (1 lineup) + 1 pro draft (3 lineups, the edit/delete e2e substrate). Finalized pro view: 32 lineups divisible by 8, field avg placement exactly 4.5 by construction (frontend-design:110).
- [ ] Failing tests: `TestSeed_LoadsFixture`, `TestSeed_ProInvariants` (counts divisible by 8, field avg 4.5), `TestSeed_RefusesNonEmpty`, `TestSeed_GoldenSpot` (one hero's n/top4/avg hand-derived from the comments).
- [ ] Implement; run `task seed` against `data/app.db` once to prove the loader, then recreate `data/app.db` migrated-empty via a fresh `Open` and commit it empty (deviation 5). Commit: `feat: shared seed fixture for dev, analytics goldens and e2e` + `chore: commit migrated empty app.db`.

---

## Stage B: sqlite stores (one agent, parallel with C and D)

**Files:** Create: `internal/store/sqlite/codex.go`, `internal/store/sqlite/match.go`, `internal/store/sqlite/matchwrite.go`; tests alongside. All tests use real sqlite files in t.TempDir with real migrations (readme:199).

### Task B.1: codex store

- [ ] CRUD quartets (`Create/Update/Delete/List`) for race, class, hero, item, relic, patch, pro. Hero variant rewrites `hero_races`/`hero_classes` junctions in one WithTx; item variant rewrites `item_recipes`; race/class variants write their tier rows. Plus `GetHeroByName(name)` (the datalist fallback lookup), `Get*` for edit pages, `ListHeroesWithStats` (plain sqlite group-by for the codex read-only columns lineups/avg place, all-time, readme layer 3 note).
- [ ] Failing tests first: `TestCodexCRUD_RoundTrip` families per entity, `TestUpdateHero_RewritesJunctions`, `TestDeleteHero_InUseMapsToErrInUse` (insert hero into a slot, delete -> `sqlite3.ErrConstraintForeignKey` mapped to `domain.ErrInUse`), `TestListHeroesWithStats_MatchesFixtureGolden` (via seed.Load).
- [ ] Map constraint errors centrally: `sqlite3.ErrConstraintForeignKey` -> `domain.ErrInUse`; `lineups` unique(match_id, placement) -> `domain.PlacementConflictError{Placement}` (the map lives here so E renders exact spec copy).
- [ ] Green. Commit: `feat: sqlite codex store with junction rewrites and constraint mapping`.

### Task B.2: match read store

- [ ] `MatchFilter{PatchID int64; Source, State string}` + `ListMatches(filter) []MatchListRow` (with lineup count and n/8 pip data), `GetMatch(id)` returning match + lineups + slots + slot items + lineup relics fully assembled for the editor and detail pages, `ListPros`, `ListPatches` for form options.
- [ ] Failing tests: `TestListMatches_FiltersPatchSourceState`, `TestGetMatch_ReturnsSlotsItemsRelics` (seed fixture, assert deep shape).
- [ ] Green. Commit: `feat: sqlite match read store with filters and deep assembly`.

### Task B.3: match write store

- [ ] Every mutation one WithTx (readme:205): `CreateMatchShell`, `AddLineup`, `CopyLineup` (clones slots+stars+slot_items+lineup_relics+label, placement = `FirstFreePlacement`, resets pro/w-d-l/networth per frontend-design:176), `UpdateLineup`, `DeleteLineup`, `AddSlot` (slot_index = `NextSlotIndex`), `SetSlotStars`, `DeleteSlot`, `AddSlotItem`, `RemoveSlotItem`, `AddLineupRelic`, `RemoveLineupRelic`, `FinalizeMatch`.
- [ ] Failing tests: `TestAddLineup_PartialFailureRollsBackAll` (inject error mid-tx via closed item fk), `TestAddLineup_DuplicatePlacementMapsToConflictError`, `TestDeleteLineup_CascadesSlotsAndItems`, `TestCopyLineup_CopiesBoardResetsScalars`, `TestFinalizeMatch_SetsFinalizedAt`.
- [ ] Green. Commit: `feat: sqlite match write store with copy and finalize`.

---

## Stage C: ui shell + http skeleton (one agent, parallel with B and D)

**Files:** Create: `internal/ui/layout.templ`, `internal/ui/components.templ`, `internal/ui/ui.go`, `internal/ui/static/app.css`, `internal/httpapi/server.go`, `internal/httpapi/middleware.go`, `internal/httpapi/render.go`; page stubs registered but rendering empty states. No store/analytics/service imports (deviation from architect plan: no ports.go; stubs keep C dependency-free).

### Task C.1: design system + layout + components

- [ ] `app.css`: full token system as css custom properties on `:root` (frontend-design:388), component classes `topbar strip lcard slot chip pip spread legend` (frontend-design:389), barlow @font-face over the vendored woff2 files, `color-scheme: dark`, the one settle animation with `prefers-reduced-motion`, focus-visible 2px gold outline, breakpoints 900/720/600. Budget under 400 sloc.
- [ ] `layout.templ`: app shell (topbar nav dashboard/matches/codex, current section gold underline, content max 1240px), codex secondary tab row.
- [ ] `components.templ`: Button (primary/quiet/destructive), Pips (10x10 gold boxes + n/8 text), PlacementNumeral (gold/chalk/slate tier split), RarityDot, StarPips (3 marks + "n/3" text), Chip (with remove-x mini form), SpreadBar (72x10 8-segment finishes bar, gold/light/dim hexes, aria-label with all 8 shares), MetricLegend, EmptyState. Numbers arrive pre-formatted; templates never compute (frontend-design:391).
- [ ] Tests: golden render-string assertions `TestPips_Golden`, `TestSpreadBar_AriaCarriesShares`, `TestBaseLayout_Golden`.
- [ ] `templ generate` before tests (Taskfile `gen` dep). Commit: `feat: app shell layout, token stylesheet, vendored assets`.

### Task C.2: http skeleton

- [ ] `render.go`: `isHX(r)` (HX-Request header), `renderPage(w, r, status, comp)`, `renderOOB(w, r, status, frags ...templ.Component)` (concatenated fragment bodies; hx-swap="none" means the body is pure oob content).
- [ ] `middleware.go`: origin guard (mutating requests must carry Origin or Referer host matching the server host, else 403 naming the cause; GETs skip), slog access log wrapper.
- [ ] `server.go`: `New(deps...)` with the full route table from readme:213-236 registered to stub handlers rendering the spec empty states (frontend-design:352-354); `GET /` redirects to `/dashboard`.
- [ ] Failing tests: `TestOriginGuard_BlocksForeignOriginPost`, `TestOriginGuard_AllowsSameOriginAndReferer`, `TestOriginGuard_SkipsGet`, `TestIsHX`, `TestRoutes_RegisterAllSpecPaths` (table over every readme route, stub 200/303, non-hx gets full page).
- [ ] Green. Commit: `feat: http skeleton with origin guard, render helpers, route table stubs`.

---

## Stage D: analytics engine (one agent, parallel with B and C)

**Files:** Create: `internal/analytics/engine.go`, `internal/analytics/query.go`, `internal/analytics/service.go`, `internal/analytics/queries/{heroes,finishes,field,synergies,items,relics,networth,count}.sql`; tests alongside, all on `seed.Load`ed temp dbs.

### Task D.1: duckdb engine, attach, shared mutex

- [ ] `New(dbPath string, mu *sync.Mutex) (Service, error)`: `sql.Open("duckdb", "")`, then `INSTALL sqlite`, `LOAD sqlite`, `ATTACH '<filepath.ToSlash(dbPath)>' AS ac (TYPE SQLITE, READ_ONLY)` (readme:186, windows backslash guard). INSTALL/LOAD failures wrap the explicit first-run-needs-network message (readme:243).
- [ ] Every batch method locks `mu` for the whole query with rows fully scanned inside the lock (readme:290).
- [ ] Failing tests: `TestEngine_AttachesFixture`, `TestWriteThenRead_DuckdbSeesCommittedSqlite` (sqlite insert via Store.WithTx, then read through the engine), `TestEngine_BatchHoldsSharedMutex` (batch body observes TryLock fail on the store's WriteMu).
- [ ] Green. Commit: `feat: duckdb engine with read-only sqlite attach behind shared mutex`.

### Task D.2: query catalogue + param expansion

- [ ] Queries read `ac.lineups`, `ac.matches` (finalized_at > 0), etc. Filter guards written as `(CAST(? AS VARCHAR) IS NULL OR m.source = CAST(? AS VARCHAR))` so absent filters pass. `heroes.sql`: view CTE over finalized lineups with guards; picks CTE joining `ac.lineup_slots`; per-hero n/top4/avgplace. `items.sql`: item lift against a same-hero baseline CTE keyed on hero_id (readme:249). `synergies.sql`: union race+class unit counts per lineup, join tier ladders, lift vs field avg where slice non-empty else fallback to field avg (readme:260 lift fallback property). `field.sql`: field avg + lineups-in-view count. `finishes`: per-entity placement histogram feeding [8]int.
- [ ] `query.go`: reads embedded sql, expands named tokens `:patch_id`/`:source` to `?` in first-appearance order (duplicating guard params); test `TestQuery_ExpandOrderStable`. If the duckdb driver's named-parameter support proves clean at implementation time, prefer `sql.Named` + `$name` and drop the expander (verify, then pick one; do not ship both).
- [ ] Green. Commit: `feat: analytics catalogue queries with filter guards`.

### Task D.3: assembly service + goldens

- [ ] `service.go`: implements `analytics.Service`; assembly applies rates, `WilsonLB` floors, vs-field deltas, finishes arrays. Empty filter results -> no rows; the handler renders the empty state.
- [ ] Failing tests (goldens hand-derived from fixture.sql comments): `TestHeroPerformance_Golden`, `TestSynergyLift_Golden`, `TestItemLift_Golden`, `TestRelicLift_Golden`, `TestNetworthByPlacement_Golden`, `TestFieldAvg_ProExactly45`, `TestLineupsInView_Filters`, rapid `TestLift_EmptySliceFallsBackToFieldAvg`.
- [ ] Green. Commit: `test: golden analytics numbers on the shared fixture`.

---

## Stage E: services + handlers wiring (one agent, needs B + C; D may finish in parallel)

**Files:** Create: `internal/service/entry.go`; `internal/httpapi/forms.go`, `internal/httpapi/matches.go`, `internal/httpapi/lineups.go`, `internal/httpapi/slots.go`, `internal/httpapi/dashboard.go`, `internal/httpapi/codex.go`, `internal/httpapi/codex_heroes.go`, `internal/httpapi/codex_items.go`, `internal/httpapi/codex_synergies.go`; `internal/ui/editor.templ`, `internal/ui/dashboard.templ`, `internal/ui/matches.templ`, `internal/ui/codexpages.templ`; Modify: `internal/httpapi/server.go` (real deps replace stubs), Create: `cmd/autochess/main.go`.

### Task E.1: entry service

- [ ] `EntryService{St *sqlite.Store}`: `CreateMatch(ctx, cmd) (int64, error)`, `AddLineup(ctx, matchID int64, cmd domain.AddLineupCmd, copyFrom int64) (EditorView, error)` (validates scalars via domain, pre-checks placement uniqueness for the friendly error, stores or copies, reloads and returns the full editor view), `FinalizeMatch(ctx, id) error` (load, `ValidateFinalize`, one tx write). `EditorView` = match + lineups deep-assembled + hero/item/relic/pro option lists, defined here and consumed by the ui oob twins. Codex stays handler->store direct (readme:207).
- [ ] Failing tests: `TestAddLineup_FriendlyPlacementConflict`, `TestFinalize_RejectsProSevenLineups`, `TestCopyLineup_BoardClonedLabelKeptPlacementNext`.
- [ ] Green. Commit: `feat: entry service for shell, lineup add and copy, finalize`.

### Task E.2: match editor handlers + the swap contract

- [ ] `forms.go`: decode helpers preserving raw values for 422 rerenders.
- [ ] Every mutation handler: non-hx -> 303; hx -> `renderOOB` with the exact fragment set from the swap table (frontend-design:274-292), minus nested fragments (deviation 3). Success: cards + pips (addform reset rides cards). 422: the issuing form's oob fragment with preserved values, inline rust errors under the first bad field, `data-autofocus` on it. All submit controls carry `hx-disable="this"`; all forms carry `hx-sync="this:abort"`. Stable ids exactly `frontend-design:272`.
- [ ] Before wiring focus hooks: grep the vendored `htmx.min.js` once for `htmx:after:swap`, `htmx:before:swap`, `hx-on:htmx:` spellings (spec's own instruction, frontend-design:392). One container listener per region focuses `[data-autofocus]` after swap; delete controls capture the neighbor into `window.__acNext` in the before phase. This is the only js beyond declarative attributes.
- [ ] Example handler shape (add lineup):

```go
func (s *Server) addLineup(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
    cmd, raw := decodeAddLineup(r)          // domain.AddLineupCmd + raw form values
    ev, err := s.Entry.AddLineup(r.Context(), id, cmd, copyFrom(r))
    if err != nil { s.lineupFormError(w, r, id, raw, err); return } // 422 oob or 303 banner path
    if !isHX(r) { http.Redirect(w, r, fmt.Sprintf("/matches/%d/edit", id), http.StatusSeeOther); return }
    renderOOB(w, r, http.StatusOK, ui.CardsOOB(ev), ui.PipsOOB(ev))
}
```

- [ ] Failing handler tests on a real Server + temp store + test-local fake analytics: `TestAddLineup_OOBResponseContainsCardsAndPips`, `TestAddLineup_422PreservesValuesAndMarksFirstError`, `TestAddLineup_PlacementConflictRendersSpecCopy`, `TestAddLineup_NonHXRedirectsToEditor`, `TestCopyLineup_ResetsProAndScalars`, `TestAddSlot_FillsLowestFreeCell`, `TestAddSlot_UnknownHeroRendersSpecCopy`, `TestDeleteSlot_ReturnsGrid`, `TestDeleteLineup_ReturnsCardsAndPips`, `TestFinalize_NonHXRedirectsToDetail`, `TestFinalize_InvalidRendersBannerNamingProblem`, `TestSaveStars_UpdatesGrid`, `TestAttachItem_CapsAtSix`, `TestRemoveRelic_RerendersRelics`.
- [ ] Green. Commit: `feat: match editor handlers honoring the oob swap contract`.

### Task E.3: codex handlers

- [ ] Shared helpers in `codex.go`: `pathID`, `formString`, `formInt`, `optionalID`, `formFields` (raw values), `codexDelete` answering hx-delete with `HX-Redirect: /<index>` and plain requests with 303 (frontend-design:218). Create/edit are plain posts: 303 on success, full-page 422 rerender with values. `codex_heroes.go` (junction fields, race/class selects populated from the synergies tab), `codex_items.go` (recipe multi-select), `codex_synergies.go` (tier ladder editor rows with per-tier save/delete, manages races and classes, frontend-design:226).
- [ ] Failing tests: `TestCreateHero_303OnSuccess`, `TestCreateHero_422RerendersWithErrors`, `TestDeleteHero_HXRedirectHeader`, `TestDeleteHero_InUseRendersConflict`, `TestSaveTier_RoundTrip`, plus one 303/422 pair for pro and patch as pattern representatives.
- [ ] Green. Commit: `feat: codex handlers with hx-delete redirect flow and synergy ladder editor`.

### Task E.4: dashboard partials + match list + match detail pages

- [ ] `dashboard.go`: `GET /dashboard` full page, `/dashboard/heroes|synergies|items|relics` partials; filter selects submit hx-get on change with `hx-include="closest form"`, `hx-target` on the panel id `dashpanel-{view}`, `hx-push-url`, `hx-sync="this:abort"` (frontend-design:109). Empty state copy verbatim. Codex and match list pages per frontend-design wireframes; match detail renders the scoreboard strip + read-only boards.
- [ ] Failing tests: `TestDashboard_FullPageVsPartialOnHXRequest`, `TestDashboard_FilterRidesQueryParams`, `TestMatchList_PipsAndStateFilters`, `TestMatchDetail_ScoreboardRankedByPlacement`.
- [ ] Green. Commit: `feat: dashboard partials wired to analytics, match list and detail pages`.

### Task E.5: main wiring

- [ ] `cmd/autochess/main.go`: flags (addr default `127.0.0.1:8080`, db default `data/app.db`, `-seed` mode), slog, `sqlite.Open`, `analytics.New(dbPath, &st.WriteMu)`, `httpapi.New`, origin guard, graceful shutdown draining http then closing duckdb then sqlite (readme:211).
- [ ] Smoke: `task dev` boots, `task seed` + reload shows the dashboard with fixture numbers. Commit: `feat: main wiring with graceful shutdown of both engines`.

---

## Stage F: e2e + adversarial review (main session + 2 agents, needs D + E)

### Task F.1: playwright e2e

**Files:** Create: `e2e/harness_test.go`, `e2e/codex_test.go`, `e2e/matchentry_test.go`, `e2e/dashboard_test.go`, `e2e/editdelete_test.go` (all `//go:build e2e`).

- [ ] Harness: per-test `net.Listen("tcp", "127.0.0.1:0")`, fresh `t.TempDir()` db, real `sqlite.Open` (migrates), `seed.Load`, real `analytics.New` + mux via `httpapi.New`, headless chromium, auto-accept dialogs for hx-confirm. `task playwright` installs browsers. If the duckdb sqlite extension cannot load (offline first run), skip analytics-dependent tests with the reason (readme risk).
- [ ] Flow 1 `TestCodexHeroFlow`: create race with tiers, create hero, appears in table, edit cost, delete -> HX-Redirect to index, row gone.
- [ ] Flow 2 `TestProMatchEntryFlow`: create pro match, add 8 lineups (placement prefill advances, pips 1/8..8/8), add heroes via datalist input, add item + save stars, use copy, submit duplicate placement -> inline error with value preserved, finalize -> 303 to detail with 8-card scoreboard.
- [ ] Flow 3 `TestDashboardRendersNumbers`: lineups-in-view count, first hero row matches a fixture golden, patch select triggers the hx-get swap and push-url, reloading the pushed url renders the full page with the filter.
- [ ] Flow 4 `TestEditDeleteFlow`: on the seeded draft, edit scalars, re-sort on placement change, save stars, remove item chip (no dialog), delete slot with confirm -> placeholder returns and next add fills the lowest cell, delete lineup -> pips drop, focus lands on the captured neighbor.
- [ ] Green under `task e2e`. Commit: `test: playwright e2e harness and the four flows`.

### Task F.2: verification chain

- [ ] In order, committing at each green step (user protocol): feature tests -> fmt -> lint -> vet -> full unit suite -> e2e. Fix failures at root cause; never weaken a test.

### Task F.3: adversarial review + fixes

- [ ] Dispatch 2 independent agents in parallel, neither seeing the other, both given the repo + both specs + this plan:
  - adv-1 (data integrity/concurrency): tx atomicity and WriteMu coverage, FK/constraint mapping completeness, migration idempotence against the committed db, finalize rules, seed invariants, WilsonLB edges, SQL vs hand-computed goldens.
  - adv-2 (ui contract): swap table vs actual stable ids, nested oob violations, hx attribute spellings vs the vendored htmx.min.js (including both hx-on event names), all 12 focus rows, 422 value preservation, verbatim copy and token conformance, accessibility floor, non-hx fallbacks.
- [ ] Triage findings, fix every confirmed one at root cause, re-run the chain from the earliest affected step. Commits: `fix: <finding>` per confirmed finding.

---

## Verification (definition of done)

1. `task check` green end to end (gen, fmt, lint, vet, unit, e2e, generated-files-clean git diff).
2. `task dev` boots on the flag addr; walking the 5 screens (dashboard, matches list, editor, codex, match detail) against `task seed` data exercises the swap contract: add/copy/delete lineup, add hero/item/relic, finalize, dashboard filters swap partials and push urls.
3. `data/app.db` committed migrated-empty; `.githooks/pre-commit` live via `core.hooksPath`; a staged or non-empty journal blocks commit (verify by hand once).
4. Both adversarial agents ran, findings triaged, confirmed ones fixed and the chain re-run.
5. Working tree clean, all commits conventional, no AI attribution lines anywhere.

## Explicit non-goals (readme:280)

Game window capture, auth/multi-user, hosting, per-patch hero stat history, public api. Out of scope for this plan; do not scaffold hooks for them.
