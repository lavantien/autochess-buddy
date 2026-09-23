# whiteboard defense

Every section here is a 2 to 5 minute whiteboard explanation of one shipped system. The "sketch" is what to draw. The "defense" answers the questions a person pulling you aside would actually ask: why this and not the obvious alternative, and what it costs. The binding specs are readme.md (architecture) and frontend-design.md (ui). This file is the spoken-word layer on top of them.

## the 30 second pitch

A single go binary a person runs locally to record autochess games by hand: a codex of races, classes, heroes, items, relics, patches and pros; a match editor that captures 8 player boards; and an analytics dashboard (pick rate, top 4 rate with a Wilson floor, avg placement, synergy/item/relic lift, networth curve) computed over finalized matches by duckdb reading the sqlite file. Server renders everything with templ. htmx swaps fragments. No client framework, no build step beyond templ generate, no auth, no hosting.

Defense: localhost single user kills whole problem classes (auth, multi tenant, scale). The interesting risk is data integrity and ui responsiveness under manual entry, which is where the effort went.

## request lifecycle

```
browser
  |
  v
logRequests (slog one line per request)
  |
  v
OriginGuard  (mutations must carry a same host Origin or Referer, else 403)
  |
  v
go 1.27 ServeMux  (method + path patterns, e.g. "DELETE /slots/{id}")
  |
  v
handler: decode form -> service or store -> templ component
  |
  +-- hx request (HX-Request header): renderOOB, 200 or 422 fragment
  +-- plain request: renderPage full page, or 303 redirect
```

Sketch this spine first for any system question. Everything else hangs off it.

Defense:
- origin guard: the app has no cookies or sessions, so classic CSRF does not apply, but the guard costs ~30 lines and stops cross site form posts from ever mutating data. GETs skip it.
- ServeMux patterns: stdlib since 1.22, no router dependency. Path values feed ids, one place to parse.
- the hx/plain split is the whole no-js story: every route works with js off (full page or 303) and feels fast with js on (fragment swap). One code path decides, `isHX`, based on one header.

## the swap contract (client facing core)

The page is a set of regions with stable ids. htmx swaps only what an action changes.

```
editor page regions:
  #cards-{id}   lineup cards grid (add form lives inside it)
  #pips-{id}    8 placement pips + n/8
  #grid-{id}    one lineup's 12 slot cells
  #heroform-{id} add hero form
  #relics-{id}  relic chips + picker
dashboard: #dashpanel-{view}, #dashcount rides along oob
```

Mutation responses return exactly the fragments the swap table names, each wrapped with `hx-swap-oob` so htmx can land them at their ids. A success add lineup returns cards + pips. The reset of the add form rides the cards re-render because the form lives inside the cards region: that is recorded deviation 3, chosen over emitting a third fragment to avoid swapping a fragment inside a fragment (htmx cannot nest oob targets inside another swapped element).

Errors: a 422 returns the issuing form with typed values preserved, the first bad field named with exact copy, and `data-autofocus` on that field. htmx swaps error responses by default, so the form rerenders in place without any client code.

Defense:
- why not react: the spec bars client frameworks. The stable id + oob design gets partial updates with zero hand written render code.
- why not json + fetch: two representations of every page to maintain. One templ source renders the full page and every fragment.
- duplicate submits: `hx-disable` on submit controls plus `hx-sync="this:abort"` per form, and the server side unique(match_id, placement) constraint as the real guarantee mapped to a friendly error.

## focus contract

After any swap, focus goes where the user's hands should be next: the just added thing, or the neighbor of a deleted thing.

```
templ side:  data-autofocus?={ cond }   (only rendered when true)
delete side: hx-on:htmx:before:swap captures a neighbor ID into
             window.__acNext: slot delete takes the previous slot
             cell, else the next, else heroform-{id}; lineup delete
             takes the next card, else addform-{id}
focus.js:    on htmx:after:swap -> resolve __acNext's id in the
             fresh dom and focus its first focusable, else focus
             [data-autofocus] and consume the attribute
```

focus.js is the only sanctioned script beyond htmx itself, served from /static with defer.

Defense:
- why ids and not node references in the capture: the delete response replaces the whole grid or cards region, so a captured node is detached by the time after:swap fires and focusing it is a no-op. An id re-resolves inside the fresh dom, which is also what the spec asks for: focus targets get their own ids.
- why the autofocus attribute is consumed on focus: a stale data-autofocus in a region the swap never touched used to win the document-wide query on a later swap and steal focus from the delete capture. Removing the attribute after focusing makes each target one-shot, so the capture wins whenever a delete set one.
- why one global listener and not per region: the spec allows one script. Document level events catch every swap regardless of which region changed.
- why external file now: it used to be inline in the layout template. templ fmt shells out to prettier for script blocks and its windows bridge passes the temp filename through cmd.exe unexpanded, so formatting crashed. Moving the script to a static file removed the only script block in the repo and unblocked formatting. Same behavior, one less toolchain dependency.
- the `?=` suffix matters: templ renders `attr={ false }` as a present attribute and html presence means true, which once made every select default to its last option. `attr?={ cond }` omits the attribute when false.

## match editor

Object model:

```
match (patch, played_at, source pro|me, finalized_at)
  lineup (placement 1..8 unique per match, label, pro, wins/draws/losses, networth)
    slot (slot_index 0..11, hero, stars 1..3, up to 6 items)
    lineup relics (chips)
```

Flows: create match shell, add lineups (placement prefills the next free cell), build boards via a free text hero input backed by a datalist (the name must exist in the codex, unknown names get "no hero named x in the codex. add it first."), copy a lineup (clones board and label, takes the next free placement, resets pro and scalars), finalize (pro needs exactly 8 lineups covering 1..8; me needs exactly 1). Finalized matches drop out of the editor and into read only history.

The finalize lock is enforced in the store, not the ui: every lineup, slot, item and relic write resolves its match inside the same tx and refuses with "this match is finalized and can no longer be edited." once the marker is set. Validation and the stamp share one tx (a lineup add cannot slip between the rule check and the marker), and a repeat finalize is idempotent, never moving the stamp. Defense: the editor redirect only hides the forms; a stale tab or a direct post could still reach the store, so the guarantee lives where the writes happen. A board cell raced by two adds maps to friendly copy instead of raw driver error text.

Defense:
- placement as the unique key: it is the scoreboard identity of an autochess match, so uniqueness is a domain rule, not a db nicety. The friendly conflict copy comes from the service pre-check; the db constraint is the backstop.
- slots fill the lowest free cell, not append: a deleted hero mid board should not shift everything. Pinned by a property test over random permutations.
- why a datalist and not a select: free text entry is faster for a known player, the datalist suggests, the codex lookup validates on submit, and adding a missing hero is one tab away.

## codex

Seven entities over plain tables plus three junction shapes: hero_races/hero_classes (1-2 lineages each), item_recipes (multi select), and tier ladders for races and classes (counts 2/4/6 map to tier names, edited as ladder rows). Deletes that history still references map the sqlite foreign key violation to a friendly "existing matches keep their history" answer: hx deletes answer 204 + HX-Redirect to the index on success, and the 409 conflict page swaps into the main region (hx-select to the page's own content) so the banner names the refusal without a navigation.

Defense:
- synergies 422 errors are scoped per ladder: field keys carry the entity id (races-3-name, races-3-tier), so a tier error renders exactly once under the ladder that failed, names the entity in its copy, and the rerender keeps the typed name, count and effect under the same keys. the name-taken copy comes from a real uniqueness pre-check over the list call, never from mapping an unknown store error to that sentence.
- junction rewrites in one tx: updating a hero replaces its lineage rows wholesale rather than diffing, simplest correct shape, rollback leaves nothing half written.
- delete in place and re add would orphan history: the spec says keep it. FK enforcement is on per connection, and the error mapping lives in the store so every caller renders the same copy.

## dashboard and analytics

```
sqlite data/app.db  (writes: Store.WithTx under WriteMu)
        ^
        | attach READ_ONLY
duckdb engine       (every batch holds the same WriteMu for the whole scan)
```

Filters: patch and source narrow the view to finalized matches only (finalized_at > 0). Metrics per hero: picks, pick rate = picks / lineups in view, top 4 rate, Wilson 95% lower bound of that rate as the "floor", avg placement, vs field delta, and an 8 segment finishes bar. Synergy and item and relic lift: avg placement of lineups containing the thing minus the field avg, with an empty slice falling back to the field average so nothing divides by zero. Item lift compares against a same hero baseline (slots of the same hero without the item) because items sit inside a hero's slot. Networth by placement closes the page.

Defense:
- two engines: sqlite is the transactional truth, duckdb is the analytical reader. Attaching read only means analytics can never write and never needs its own storage. The shared mutex serializes reads against write transactions so duckdb never sees a half committed file.
- Wilson lower bound instead of a raw rate: with small n a 100% top 4 on 3 games must not outrank 55% on 40. The floor rises with n at a fixed rate, which a property test pins. The math runs in Go on counts SQL returns: sql does all aggregation, Go applies one deterministic scalar (recorded deviation 2).
- why duckdb at all: the lift and histogram queries are analytics shaped. Writing them as sqlite sql was possible but the spec asked for duckdb and the attach design keeps a single source of truth.

## data layer decisions

- read-model rows and filters (hero and synergy and item and relic and placement rows, the match list row, hero stats) live in domain next to the entity types they embed, so the ui templates render against them without importing the store or analytics packages. That honors "each layer only talks to the layer directly below" (readme:100), which a type-only import from a template would silently break.
- journal mode TRUNCATE: the rollback journal sits at a stable path and is zero bytes when idle, which the pre-commit hook checks so a torn db can never be committed.
- goose migrations embedded in the binary: no migration tool needs installing, a fresh db is one `Open` away.
- data/app.db committed migrated-empty and seeding is opt-in (`make seed`): fake fixture data must never mix with a person's real entry history.
- every mutation is one WithTx: multi table writes (a lineup with slots and items and relics) either land whole or not at all. A test injects a failure mid tx and proves the rollback.
- constraint mapping in the store layer: FK violations become ErrInUse, the placement unique becomes PlacementConflictError, driver error text never reaches a user.

## the one weird workaround (know this cold)

This prebuilt duckdb go driver crashes the process on any bound parameter on this windows toolchain, while parameterless statements run clean. Filters therefore inline as literals: PatchID is always an int formatted with %d, Source arrives from a handler fed select and is wrapped by quoteLiteral which doubles embedded quotes, and each embedded query carries exactly one {{filters}} token so the substitution cannot double apply. A hostile source string test pins that the attack vector is inert.

Defense: the alternative was abandoning bound parameters entirely or building duckdb from source. Literal inlining with typed inputs plus quote escaping is safe here because no free text ever reaches a filter: both values come from the ui's own selects. The invariant and the crash rationale are documented at the top of internal/analytics/query.go. Cost if the environment changes: a bind capable build makes this removable in one function.

## tooling and release

- make is the single entry point: build gen fmt lint vet test e2e playwright seed serve uiwatch dev shot check. Everything, including targeted runs (`make test PKG=./internal/analytics RUN=-run=X`), goes through it so CGO_ENABLED=1 and flags never drift.
- `make check` is the full chain ending in a git diff guard that stale generated templ code cannot land.
- `make shot` is the release gate: boots the seeded app headlessly, screenshots the dashboard, rewrites the marked image block at the top of readme.md. Run before every github release.
- scripts/firewall.ps1 adds (or with -Remove deletes) an inbound windows firewall allow rule for the built exe, needed only if serving beyond loopback (`-addr 0.0.0.0:8080`).
- playwright e2e covers four flows: codex hero lifecycle, pro match entry to finalize, dashboard numbers and filters, edit and delete. They found 4 real bugs before shipping (missing oob flags, the templ presence attribute trap, a missing delete control, a stale count).

## what we did not build, on purpose

Auth and multi user (localhost single user), game window capture (manual entry is the point), hosting, public json api, per patch stat history. Saying no to these is why the whole thing is one binary and one db file.

## numbers worth having ready

2 patches, 3 races, 3 classes, 12 heroes, 8 items, 4 relics, 4 pros, 6 seeded matches (4 pro finalized, 1 me finalized, 1 draft for editing). Default analytics view: 33 lineups, field avg placement 145/33. 7 unit test packages, 4 e2e flows, 0 lint issues. Go 1.27, templ v0.3.1020, htmx 4.0.0 vendored, duckdb-go v2.10505.0 (duckdb 1.5.5), goose v3.28.0, sqlite driver v1.14.52, playwright-go v0.6201.1.
