# autochess companion

## tech stack

- sqlite for transactional queries
- duckdb for analytical queries
- go 1.27+ + templ + htmx 4
- tdd + dag plan for parallel development (max 4 agents) + e2e run each feature with fake data but full interactions & adversarial review agents & regression prevention. we move slow but guarantee
- the architecture should be the simplest KISS that can satisfies the requirements but still DRY, everything should be bottom-up and from first principles
- verified pins (2026-09-23): go 1.27, templ v0.3.1020, htmx 4.0.0 vendored (npm next tag, latest still 2.x), github.com/duckdb/duckdb-go v2.5+ bundling duckdb 1.5.x, mattn/go-sqlite3, pressly/goose/v3 embedded, pgregory.net/rapid, playwright-community/playwright-go

## purpose

- since there's no online database for autochess (mobile/standalone steam) and all the wikis are severely outdated so this will be a place for me to, at first, manually enter the information regarding races/classes, heroes, items, relics, etc.
- my personal match history, again, ui for enter manually for now, and accumulative lineup history where i check match history of top leaderboard pros and enter 8 line up for each of their match in (final 12 heroes deck, relics taken, w-d-l ratio, networth, items taken for each hero)
- analytical dashboard where it use maths, algs, and data mining techniques to give me insights into the meta and the objective performance of heroes or synergies/lineups or relics or itemization
- we can look into automating info capture from game window if possible, but for this first mvp we need a complete skeleton so manual data entry is expected, and database files also get committed

## architecture

### preliminary design

#### overview

one process. sqlite is the only persistent store and the source of truth, duckdb is a derived read-only view over it. html is server rendered by templ, htmx swaps partials, there is no client framework and no json api.

```mermaid
flowchart LR
    subgraph browser
        B[html + htmx 4]
    end
    subgraph app[single go binary, cmd/autochess]
        H[net/http mux, internal/httpapi]
        S[services, internal/service]
        R[sqlite stores, internal/store/sqlite]
        A[analytics, internal/analytics]
        T[templ ui, internal/ui]
        DB[(data/app.db, sqlite)]
        D[(duckdb, in process)]
    end
    B -- "hx-get / hx-post / hx-delete" --> H
    H --> S
    S --> R
    S --> A
    R --> DB
    A -- "attach read only" --> DB
    A --> D
    H --> T
    T -- "html partials" --> B
```

#### e2e sequence flow

data entry:

```mermaid
sequenceDiagram
    participant U as user
    participant H as httpapi
    participant S as service
    participant R as sqlite store
    U->>H: POST /matches (patch, source)
    H->>S: CreateMatchShell(cmd)
    S->>R: insert draft match
    H-->>U: redirect to /matches/{id}/edit
    U->>H: POST /matches/{id}/lineups (one card form: placement, label, wdl, networth, hero rows, items, relics)
    H->>S: AddLineup(cmd)
    S->>S: domain validation (slot cap, stars, item cap)
    S->>R: insert lineup + slots + slot_items + lineup_relics
    H-->>U: htmx swap, lineup card appended, n/8 badge updates
    U->>H: POST /matches/{id}/finalize
    H->>S: FinalizeMatch(id)
    S->>S: exactly 8 distinct placements (pro) or 1 lineup (me)
    S->>R: one tx: mark finalized
    H-->>U: redirect to match detail
```

analytics read:

```mermaid
sequenceDiagram
    participant U as user
    participant H as httpapi
    participant A as analytics service
    participant D as duckdb
    U->>H: GET /dashboard/heroes?patch=x
    H->>A: HeroPerformance(filter)
    A->>D: catalogue sql over attached ac.* tables
    D-->>A: aggregate rows
    A-->>H: view model
    H-->>U: templ partial table swap
```

### detailed design

bottom-up, each layer only talks to the layer directly below, max 400 sloc per file.

#### layer 0: data

- data/app.db, sqlite, journal_mode=TRUNCATE (no wal), committed to git. journal mode is per connection, so the pool opener sets the pragma on every conn. no sidecar files exist; commit the db while the app is closed or idle, and a git pre-commit hook rejects a stray app.db-journal.
- migrations are embedded numbered .sql files applied automatically at boot, one tx each (goose v3, programmatic only). no manual migration steps ever, and the app has no docker services at all.

```sql
-- v1 schema
CREATE TABLE patches (
  id INTEGER PRIMARY KEY, version TEXT NOT NULL UNIQUE, released_at TEXT NOT NULL);
CREATE TABLE races (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE race_tiers (
  race_id INTEGER NOT NULL REFERENCES races(id),
  count INTEGER NOT NULL, effect TEXT NOT NULL,
  PRIMARY KEY (race_id, count));
CREATE TABLE classes (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE class_tiers (
  class_id INTEGER NOT NULL REFERENCES classes(id),
  count INTEGER NOT NULL, effect TEXT NOT NULL,
  PRIMARY KEY (class_id, count));
CREATE TABLE heroes (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
  cost INTEGER NOT NULL CHECK (cost BETWEEN 1 AND 5),
  ability TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '');
CREATE TABLE hero_races (
  hero_id INTEGER NOT NULL REFERENCES heroes(id) ON DELETE CASCADE,
  race_id INTEGER NOT NULL REFERENCES races(id),
  PRIMARY KEY (hero_id, race_id));
CREATE TABLE hero_classes (
  hero_id INTEGER NOT NULL REFERENCES heroes(id) ON DELETE CASCADE,
  class_id INTEGER NOT NULL REFERENCES classes(id),
  PRIMARY KEY (hero_id, class_id));
CREATE TABLE items (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
  tier INTEGER NOT NULL,
  effect TEXT NOT NULL DEFAULT '');
CREATE TABLE item_recipes (
  result_id INTEGER NOT NULL REFERENCES items(id),
  component_id INTEGER NOT NULL REFERENCES items(id),
  PRIMARY KEY (result_id, component_id));
CREATE TABLE relics (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
  effect TEXT NOT NULL DEFAULT '');
CREATE TABLE pros (
  id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
  handle TEXT NOT NULL DEFAULT '', peak_rank TEXT NOT NULL DEFAULT '');
CREATE TABLE matches (
  id INTEGER PRIMARY KEY,
  patch_id INTEGER NOT NULL REFERENCES patches(id),
  played_at INTEGER NOT NULL,
  source TEXT NOT NULL CHECK (source IN ('me','pro')),
  notes TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL);
CREATE TABLE lineups (
  id INTEGER PRIMARY KEY,
  match_id INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  pro_id INTEGER REFERENCES pros(id),
  label TEXT NOT NULL,
  placement INTEGER NOT NULL CHECK (placement BETWEEN 1 AND 8),
  wins INTEGER NOT NULL DEFAULT 0, draws INTEGER NOT NULL DEFAULT 0,
  losses INTEGER NOT NULL DEFAULT 0, networth INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  UNIQUE (match_id, placement));
CREATE TABLE lineup_slots (
  id INTEGER PRIMARY KEY,
  lineup_id INTEGER NOT NULL REFERENCES lineups(id) ON DELETE CASCADE,
  hero_id INTEGER NOT NULL REFERENCES heroes(id),
  slot_index INTEGER NOT NULL CHECK (slot_index >= 0),
  stars INTEGER NOT NULL CHECK (stars BETWEEN 1 AND 3),
  UNIQUE (lineup_id, slot_index));
CREATE TABLE slot_items (
  slot_id INTEGER NOT NULL REFERENCES lineup_slots(id) ON DELETE CASCADE,
  item_id INTEGER NOT NULL REFERENCES items(id));
CREATE TABLE lineup_relics (
  lineup_id INTEGER NOT NULL REFERENCES lineups(id) ON DELETE CASCADE,
  relic_id INTEGER NOT NULL REFERENCES relics(id),
  PRIMARY KEY (lineup_id, relic_id));
CREATE INDEX idx_slots_lineup ON lineup_slots(lineup_id);
CREATE INDEX idx_slots_hero ON lineup_slots(hero_id);
CREATE INDEX idx_lineups_match ON lineups(match_id);
CREATE INDEX idx_slot_items_item ON slot_items(item_id);
CREATE INDEX idx_hero_races_race ON hero_races(race_id);
CREATE INDEX idx_hero_classes_class ON hero_classes(class_id);
```

notes: timestamps are utc unixepoch integer seconds. heroes carry 1 to 2 races and 1 to 2 classes through the junction tables because multi lineage pieces exist and the codex is hand-entered and evolving; synergy queries run uniformly over both junctions. pro identity sits on the lineup, not the match, because several tracked pros can share one lobby. duplicate heroes on one board are legal so there is no unique on (lineup, hero), and slot_items has no position or primary key so the same item can stack. slot cap 12, items per slot cap, pro match has exactly 8 lineups, my match has 1: these are domain rules in layer 1, the schema only carries sanity checks for stable invariants (cost, stars, placement).

duckdb read contract: journal_mode=TRUNCATE keeps the main file self-consistent after every write, so duckdb attaches data/app.db read only (`ATTACH 'data/app.db' AS ac (TYPE SQLITE, READ_ONLY)`) and simply always reads current committed state. no wal, no checkpoint dance, no staleness window.

#### layer 1: domain

pure go types mirroring the tables plus all validation: slot cap 12, star range 1 to 3, item cap per slot (default 6, verify in game), placement range 1 to 8, distinct placements within a match, hero carries 1 to 2 races and 1 to 2 classes, played_at utc unixepoch parsing. drafts rest with any lineup count, finalize enforces the source rule: 1 lineup for my matches, exactly 8 with distinct placements 1 to 8 for pro matches. stdlib imports only. this is where property tests live. no sql, no http.

#### layer 2: stores

concrete sqlite types with database/sql and mattn/go-sqlite3, no interface ceremony until a second implementation exists:

- Codex: races, classes, heroes, items, relics, patches, pros crud, called directly by handlers
- MatchStore: draft shell, lineups, slots, slot_items, lineup_relics, finalize, written in one tx
- helpers: WithTx, migration runner bootstrap
- store tests run against real sqlite files in t.TempDir, never :memory:, so journal mode and locking behave as production

#### layer 3: services

thin, only where more than a single store call happens:

- entry service: create shell match, append lineup cards one form at a time, finalize enforces the source lineup counts and distinct placements in one tx
- analytics orchestration: apply filters (patch, source), run catalogue queries, map rows to view models. all math stays in sql, go only assembles
- codex crud goes handler to store directly, no pass-through service

#### layer 4: http

net/http with go 1.27 method patterns, html only. hx-request header present means return a templ partial, otherwise full page or redirect. slog for access logs and errors. config is flags with defaults (addr, db path). one tiny middleware: mutating requests must carry an origin or referer host matching the server host, because a localhost app is still csrf-able from any browser tab. graceful shutdown drains http and closes both db handles. no middleware framework.

| method | path | purpose |
|---|---|---|
| GET | / | dashboard landing |
| GET/POST | /heroes, /heroes/{id} | codex crud (same pattern for races, classes, items, relics, patches, pros) |
| GET | /matches | list, filters, n/8 draft badge |
| GET/POST | /matches/new | create shell, redirect to edit |
| GET | /matches/{id} | detail |
| GET | /matches/{id}/edit | lineup editor |
| POST | /matches/{id}/lineups | add one lineup card |
| POST | /matches/{id}/finalize | lock counts and placements |
| POST | /lineups/{id}/slots | add hero slot |
| POST | /slots/{id}/items | attach item |
| POST | /lineups/{id}/relics | attach relic |
| DELETE | /slots/{id}, /lineups/{id} | remove with htmx swap |
| GET | /dashboard | overview |
| GET | /dashboard/heroes | hero performance partial |
| GET | /dashboard/synergies | race and class lift partial |
| GET | /dashboard/items, /dashboard/relics | lift partials |

#### layer 5: ui

templ base layout, page templates, small components in internal/ui. htmx 4.0.0 vendored as a static file since npm latest still points at 2.x. one hand written dark stylesheet, tables and forms only. full frontend spec with tokens, wireframes, and the htmx swap map: frontend-design.md. lineup entry is incremental: each lineup card is its own small form so placement conflicts surface per card through the unique constraint and no 100-field atomic submit exists, a duplicate-lineup button copies the previous card since adjacent placements share most pieces, match lists show n/8 so partial entry is a resting state, stars default to 2. hero picker is a select with datalist search, no per-keystroke server calls, no client js beyond htmx. deletes use hx-delete with hx-confirm.

#### layer 6: analytics

duckdb via github.com/duckdb/duckdb-go (new home since v2.5.0, marcboeker is archived), pinned. boot installs and loads the sqlite extension explicitly and fails fast with a clear offline message instead of a mid-request error, since the first run needs network to fetch the extension. the read-only attach comes from layer 0. rollback mode lets a duckdb shared read block go writes, so every analytics batch runs behind one mutex shared with the write path, and sqlite conns set busy_timeout. the query catalogue is embedded .sql files with named filter params. the whole engine hides behind the analytics service interface: the data volume is small enough that plain sqlite group-bys would compute these metrics too, duckdb is in the stack by mandate, and the interface keeps a future swap to sqlite-only a one-layer change. metrics:

- pick rate: lineups containing the hero over all lineups, patch filtered
- top4 rate: placement <= 4 as the win proxy, always shown with the wilson 95 percent lower bound
- avg placement
- synergy lift: for a race or class at count k, avg placement of lineups with >= k units minus the field avg placement, negative is better
- item lift: avg placement of slots holding the item minus avg placement of the same hero without it
- relic lift: same shape over lineup_relics
- networth by placement curve

#### layer 7: tooling and build

taskfile targets: dev (templ generate --watch plus go run), test (unit + property), e2e (playwright-go), lint (golangci-lint), fmt (gofmt plus templ fmt), check (fmt, lint, vet, test, e2e), seed (fake data loader shared by dev and e2e). CGO_ENABLED=1 is required everywhere (sqlite + duckdb), pwsh 7 on windows with a mingw-w64 toolchain (w64devkit or msys2) as the documented prerequisite.

#### layer 8: testing and parallel development

- unit: table driven per layer, domain is the heaviest
- property: pgregory.net/rapid, synergy counts invariant under slot permutation, wilson lower bound monotone in n, lift falls back to the patch field average when a slice is empty, form decode round trips
- store tests: temp file sqlite with real migrations, crud round trips, tx atomicity, cascade deletes
- one shared sql fixture seed feeds both analytics golden tests and e2e fakes
- analytics tests: the shared fixture attached in duckdb, golden expected numbers, plus a write-then-read test proving duckdb sees committed sqlite state
- e2e: playwright-go headless chromium against the shared fixture, 4 flows: create codex hero, enter a full pro match through the lineup editor, dashboard renders computed numbers, edit and delete
- verification chain per commit: feature tests, fmt, lint, vet, full unit suite, e2e, then 2 independent adversarial review agents, fix confirmed findings, re-run

dev dag, max 4 concurrent agents:

```mermaid
flowchart TD
    A[scaffold + domain + migrations + shared seed fixture] --> B[sqlite stores]
    A --> C[ui shell + http skeleton]
    A --> D[analytics on the shared fixture]
    B --> E[services + handlers wiring]
    C --> E
    E --> F[e2e + adversarial review]
    D --> F
```

#### non-goals for mvp

game window capture (ocr or memory reading), auth and multi user, hosting, per patch hero stat history, public api.

#### risks

- htmx 4.0.0 ships under the npm next tag: vendor the exact minified file in repo
- duckdb sqlite extension downloads on first attach: accept one time download, or vendor the extension binary later
- cgo on windows needs a c compiler: prerequisite documented (w64devkit or msys2)
- committed sqlite binary churn: commit while the app is closed or idle, pre-commit hook guards a stray journal, binary diff noise is accepted
- concurrent file access: rollback mode lets a duckdb read and a go write fight over locks; every analytics batch runs behind the shared mutex from layer 6, and the analytics tests write-then-read to catch it if it ever surfaces
