# frontend design

## brief

autochess companion is a localhost tool for one person: a player of auto chess (drodo) who enters codex data by hand, logs pro match lineups, and reads the meta off computed numbers. the frontend has exactly 2 jobs: make manual entry fast enough to keep up with a match queue, and make analytics scannable at a glance.

hard constraints from the architecture (readme layer 5):

- server rendered templ components, htmx 4 vendored, no client js beyond htmx and its own attribute hooks
- one hand written dark stylesheet, tables and forms only, no css framework, no build step
- hero picker is an input with datalist, no per keystroke server calls
- incremental lineup entry, every control on a lineup card is its own small sibling form
- deletes use hx-delete with hx-confirm

## direction

the design borrows the game's own vernacular. auto chess is an 8 player auto battler with a dark stone board, a gold economy, a cost 1 to 5 rarity ladder, and a post match scoreboard that ranks 8 players. the match detail page opens with that scoreboard: a compact strip of 8 summary cards ranked by placement. the editor works vertically, a 2 column grid of full lineup cards in placement order with big placement numerals, because entry is sequential work and horizontal scrolling between forms would cost more than the motif earns. placement is a real ordinal, the numerals are data, not decoration. every other screen stays quiet: dense tables on flat bordered panels.

principles:

- color carries game meaning only. gold is the single accent (economy, victory, 5 cost, primary actions). the rarity ladder is the only multicolor system and it marks hero identity and cost, nothing else
- tables are the design. the dashboard, the codex, and the match list are ranked tables, no chart library, bars are css widths inside cells
- entry speed beats polish. defaults match the common case: stars 2, placement prefills the first free slot, the copy button duplicates the previous board, placement conflicts surface on the card that caused them, and every mistake has a local correction path
- one accent moment: the scoreboard strip. everything else stays flat

## tokens

### color

| token | hex | use |
|---|---|---|
| ink | #12161C | page background, dark blue slate like the board stone |
| panel | #1A212A | raised surfaces: cards, forms, table rows on hover |
| line | #2A3542 | 1px borders, table rules |
| chalk | #E9E4D6 | primary text, warm white like the white pieces |
| slate | #9AA4B0 | muted text, table headers, secondary labels |
| gold | #D9A441 | the accent: primary buttons, active nav, 1st place, 5 cost |
| rust | #D08075 | destructive actions and field error text |

rarity ladder, semantic only, always paired with the cost digit:

| cost | hex |
|---|---|
| 1 | #9AA3AE |
| 2 | #6FA96B |
| 3 | #5B8FD9 |
| 4 | #9B6FD9 |
| 5 | #D9A441 (gold) |

one placement coding: the tier split (1st, 2 to 4, 5 to 8) is identical everywhere. numerals and summary cards render gold, chalk, slate. bars render gold plus the precomputed hexes light #7F8892 and dim #656F7B, never opacity blends.

computed contrast, 2 decimals: chalk on ink 14.29:1, gold on ink 8.07:1, slate on panel 6.42:1, ink on gold 8.07:1, rust on ink 6.07:1, rust on panel 5.43:1. all text pairs pass wcag aa, chalk gold and rust pass aaa for large text. the bar hexes clear the 3:1 graphics minimum on ink: light 5.05:1, dim 3.55:1.

### type

2 roles, one family, distinct through width:

- barlow condensed 600 and 700: page titles, placement numerals, nav, badges, buttons, table headers
- barlow 400 and 500: body, forms, table cells, chips

vendored woff2 latin subsets next to the vendored htmx file, `@font-face` in the stylesheet, no cdn, offline safe, OFL licensed. verified on google fonts 2026-09-23.

scale:

| step | px | face |
|---|---|---|
| title | 32 | condensed 600 |
| section | 22 | condensed 600 |
| subhead | 16 | barlow 500 |
| body | 15 | barlow 400 |
| table | 14 | barlow 400 |
| small | 13 | barlow 400 |

placement numerals run 28 condensed 700. numeric columns are right aligned with fixed decimals formatted in go (percentages 1 decimal, placements 1 decimal, networth integer). no separate mono face, alignment comes from right alignment plus fixed formatting.

### space and shape

4px base grid, steps 4, 8, 12, 16, 24, 32. radius 3px on buttons and chips, 0 on tables and panels. no box shadows anywhere, hierarchy comes from border and background steps. panels are 1px line borders on panel fill, the page stays ink.

## app shell

```
+----------------------------------------------------------------------+
| autochess companion           dashboard   matches   codex            |
+----------------------------------------------------------------------+
|                                                                      |
|   content, max 1240px, left aligned, 24px gutters                    |
|                                                                      |
+----------------------------------------------------------------------+
```

slim top bar, condensed 600 nav, current section underlined in gold. codex pages add a secondary tab row: heroes, synergies, items, relics, patches, pros. no footer, no side rail. `color-scheme: dark` on root so native selects and datalists render dark.

## screens

### dashboard

```
patch [7.5 v]  source [pro v]          120 lineups in view

hero performance

hero           cost  pick    top 4   floor   avg   vs field  finishes
sky breaker     5    41.7%   63.4%  55.1%   3.8    -0.7     [#::::::.]
grim jaw        4    38.3%   60.2%  53.8%   4.5     0.0     [##:::::.]
lord of sand    2    20.8%   48.9%  44.0%   5.2    +0.7     [#::::::.]
```

- the filter row submits with hx-get on change, targets the table panel (`hx-target`, `hx-push-url`), so the url keeps the filter and the table swaps as a partial. each select carries `hx-include="closest form"` so sibling filters ride along, and the row carries `hx-sync` with the abort strategy so a slow patch response cannot overwrite a newer source response. this is the read path the readme partial routes exist for (`/dashboard/heroes?patch=7.5&source=pro`)
- sample numbers honor the invariants: a pro filter yields a lineup count divisible by 8 and a field average of exactly 4.5, so every mock shows one
- the finishes cell is a 72 by 10px bar of 8 segments, one per placement, segment width is the share of finishes at that placement, colors from the single placement coding (gold, light, dim). each bar carries an aria-label with all 8 shares
- a metric legend sits under the section title, always visible, plain words (see copy)
- the synergy table is one row per (synergy, tier), because lift is a function of the tier count:

```
synergy   tier  lineups  lift   finishes
warrior   2     98       -0.3   [#::::::.]
warrior   4     61       -0.8   [##:::::.]
warrior   6     12       -1.1   [###::::.]
human     2     88       +0.2   [#:::::::]
```

- item and relic tables repeat the row shape: name, sample size, lift, finishes bar
- networth by placement is a compact table at the bottom: placement, average networth, n. it closes the readme metric list

```
place  avg networth  n
1      63            15
2      61            15
...
```

- empty state when a filter has no finalized matches: "no finalized matches for this filter yet. finalize a few matches first."

### matches list

```
matches                                                    [new match]

patch [all v]  source [all v]  state [all v]

state   played             patch  source  lineups         notes
draft   2026-09-22 20:14   7.5    pro     [###.....] 3/8  vic lobby
final   2026-09-21 21:02   7.5    me      [#]        1/1  ranked grind
final   2026-09-20 22:40   7.4    pro     [########] 8/8  ghost cup
```

- the filter row is a plain GET form, filters ride query params
- the pips component shows one box per required lineup, 8 for pro matches, 1 for my matches, filled per entered lineup, with the n/8 count as text next to it
- draft rows link to the editor, final rows link to the match detail
- new match goes to a small form: patch select, source select (me, pro), played at datetime local, notes, then [create match]

### match editor

```
edit match  patch 7.5  source pro       [###.....] 3/8  [finalize match]

+-------------------------------+  +-------------------------------+
| 1   vic     [edit]            |  | 2   ghost   [edit]            |
| w-d-l 7-2-1      nw 61        |  | w-d-l 6-3-1      nw 58        |
| board grid                    |  | board grid                    |
| [copy]            [delete]    |  | [copy]            [delete]    |
+-------------------------------+  +-------------------------------+
                                               +-------------------------------+
                                               | add lineup                    |
                                               | placement [ 4 ]  pro [none v] |
                                               | label      [             ]    |
                                               | w [ ]  d [ ]  l [ ]  nw [  ]  |
                                               | [add lineup]                  |
                                               +-------------------------------+
```

- cards sit in a 2 column grid on wide viewports, placement ordered because the server renders the order, 1 column below 900px. the add card is the last grid cell
- the add card form collects everything the schema needs: placement (prefilled with the first free placement), pro select with a none option, label, wins, draws, losses, networth, [add lineup]
- display name precedence: the pro name when a pro is set, otherwise the label
- each lineup card carries a copy button that posts to the same add route with `copy_from` set. the server clones heroes, stars, items, relics, and label, sets placement to the first free slot, and resets pro, w-d-l, and networth to defaults. adjacent placements share boards, not players
- the edit disclosure on each card opens the same scalar fields prefilled ([save lineup]) for after the fact fixes
- the header pips and n/8 count update after every add and delete
- finalize is a plain form post, no htmx: it is the terminal action, the server answers 303 to the match detail, the browser navigates. on rule failure the editor rerenders with the error banner naming the missing lineups or the duplicate placement

### lineup card anatomy

```
+---------------------------------------------------------------+
| 1   vic  peak: king                        [edit]              |
|     w-d-l 7-2-1      networth 61                               |
|     relics  [cursed blade x] [war horn x] [+ relic]            |
| +------------+------------+------------+------------+          |
| | o grim jaw    * * - 2/3 | | o dusk ranger   * - - 1/3        |
| |   [void stone x] [fist x]| |   empty slot placeholder         |
| |   + slot details        | |                                  |
| +------------+------------+------------+------------+          |
|   grid is 4 x 3, 12 cells, empty cells render as dashed        |
|   placeholders, filled cells never move position               |
| add hero: [type a hero name] stars [2 v]  [add hero]           |
| [copy]                                              [delete]   |
+---------------------------------------------------------------+
```

- the slot grid is 4 columns by 3 rows and always renders 12 cells. empty cells are static dashed placeholders. filled cells stay in their cell position until deleted, which returns the placeholder. a new hero takes the lowest free cell index, so a deleted middle cell is refilled by the next add
- the add hero row pins to the card bottom: input with datalist filtered by the browser, stars select defaulting to 2, [add hero]. it is the single add hero mechanism
- every control on the card is its own small sibling form element, never nested, each carrying its own ids as hidden inputs
- each filled slot has one slot details disclosure (native `details`) holding all slot operations: stars select with [save stars], item select with [add item], and the item chips each with a remove x. no js
- star level renders as 3 pip marks plus the text "n/3" beside them, so the level survives low vision and screen readers
- the relics row holds relic chips, each with a remove x, and one details disclosure with a relic select, [add relic]
- copy and delete sit in the card footer, delete asks first (see copy)

### codex

```
heroes                                                    [+ new hero]

name          cost  races        classes     lineups  avg place
grim jaw       4    human        warrior       38      4.0
dusk ranger    3    elf, beast   hunter        21      4.4
```

- codex create and edit are plain full page posts with 303 redirects, no htmx, matching the handler to store path in the architecture. codex deletes use hx-delete with hx-confirm and the server answers with the `HX-Redirect` response header to the codex index: html forms cannot issue a delete method, and an htmx delete needs a navigation answer rather than a swap
- field lists, one per entity, straight from the schema:
  - hero: name, cost 1 to 5, race 1 plus optional race 2, class 1 plus optional class 2, ability, notes. the race and class selects are populated by the synergies tab, so a fresh install creates races and classes first
  - item: name, tier, effect, components (multi select over items, the recipe picker)
  - relic: name, effect
  - patch: version, released at
  - pro: name, handle, peak rank
  - race and class: name, managed on the synergies tab
- the synergies tab manages races and classes, not just displays them. each entry is a tier ladder editor: count, effect, per tier [save tier] rows, plus [delete tier]

```
races                        classes
human                [edit]  warrior               [edit]
  2   all humans +10% atk    2   warriors +30 armor
  4   all humans +25% atk    4   warriors +70 armor
  6   all humans +45% atk    6   warriors +120 armor
```

- `+ new hero` opens a `details` create form above the table with the hero fields, [save hero]
- each row links to an edit page (`/heroes/{id}`) with the same form prefilled plus [delete hero]
- codex tables show read only analytics columns (lineups, avg place) so the codex doubles as a slow dashboard for entry sanity

### match detail

```
match 14  patch 7.5  pro  finalized 2026-09-22

+----+ +----+ +----+ +----+ +----+ +----+ +----+ +----+
|  1 | |  2 | |  3 | |  4 | |  5 | |  6 | |  7 | |  8 |
| vic| |ghst| |huan| | ... placement summary cards ... |
|61nw| |58nw| |49nw| |    | |    | |    | |    | |    |
+----+ +----+ +----+ +----+ +----+ +----+ +----+ +----+

(boards below, read only, same anatomy as the editor, no forms)

notes  vic lobby, ghost went 8th rolling warriors
```

- the scoreboard strip is the accent moment: 8 compact summary cards (placement numeral, display name, w-d-l, networth) in one row, ranked, no boards. below 900px it wraps to 2 rows of 4, below 600px to 2 columns
- the full boards below render read only in the same 2 column grid as the editor

## the swap contract

htmx 4 facts this relies on, verified 2026-09-23: error responses swap by default, `hx-swap-oob` still exists and the main swap runs before oob swaps, `hx-confirm` is still core, attribute inheritance is opt-in so every interaction attribute goes on the element itself, `hx-disabled-elt` was renamed to `hx-disable`, and the `HX-Redirect` response header remains the core full navigation mechanism.

one rule covers every mutation, so there is exactly one mechanism to wire and review:

- every mutating control posts with `hx-swap="none"`
- the response is a set of out of band `outerHTML` fragments. each fragment carries the stable id of the region it replaces. the server owns rendered state: it re-sorts the cards, resets the issuing form, refreshes the pips, and redraws the affected grid
- fragments never nest: when a response replaces an outer region (cards), the fragments for regions inside it (card, heroform, grid) are omitted
- on success the issuing form rerenders reset (placement prefill advances, datalist input clears)
- on 422 the issuing form rerenders with entered values preserved and inline errors under the first bad field. field-less forms (copy) render their error inline beside the button
- double submits are guarded twice: every submit control carries `hx-disable="this"` and every form carries `hx-sync` with the abort strategy, so an enter keypress in a text field cannot race the button. the placement unique constraint backs lineup adds server side, duplicate heroes on one board are legal data

stable ids: `cards-{matchId}`, `pips-{matchId}`, `addform-{matchId}`, `card-{lineupId}`, `editform-{lineupId}`, `heroform-{lineupId}`, `grid-{lineupId}`, `relics-{lineupId}`, `slot-{slotId}`, `dashpanel-{view}`. every oob fragment uses the id of the element it replaces.

| action | request | oob fragments in the response | focus after |
|---|---|---|---|
| add lineup | POST /matches/{id}/lineups | cards, pips, addform (reset) | new card add hero input |
| copy lineup | POST /matches/{id}/lineups + copy_from | cards, pips, addform (reset) | new card add hero input |
| edit lineup scalars | POST /lineups/{id} | card and pips, cards instead of card when placement changed | edit disclosure |
| add hero | POST /lineups/{id}/slots | grid, heroform (reset) | heroform input restored |
| save stars | POST /slots/{id} | grid | slot details disclosure |
| add item | POST /slots/{id}/items | grid (details rendered open) | item select |
| remove item | DELETE /slots/{id}/items/{itemId} | grid | slot details disclosure |
| add relic | POST /lineups/{id}/relics | relics | relic select |
| remove relic | DELETE /lineups/{id}/relics/{relicId} | relics | relic disclosure |
| delete slot | DELETE /slots/{id} | grid | previous focusable in the card |
| delete lineup | DELETE /lineups/{id} | cards, pips | next card, else the add form |
| validation error | any mutation above | the issuing form with values and errors | first errored field |
| create match | POST /matches/new | none, plain post, 303 redirect | n/a |
| codex save | POST codex routes | none, plain post, 303 redirect | n/a |
| codex delete | DELETE /heroes/{id} and siblings | none, hx-delete with hx-confirm, HX-Redirect to the codex index | n/a |
| finalize | POST /matches/{id}/finalize | none, plain post, 303 redirect | n/a |

focus movement is part of the contract, not decoration: htmx swaps that destroy or replace the focused control drop focus to the body. focus targets get their own ids (the `heroform-{lineupId}` input, the first errored field, the captured neighbor). the move runs in the after-swap phase for oob content, wired with htmx's own `hx-on` hooks. the 2 delete rows capture the neighbor in the before-swap phase, before the node is gone. this is the only permitted wiring beyond declarative attributes.

non-hx requests to any route get the full page or a redirect, per the readme http contract.

## components and states

- buttons: primary (gold fill, ink text, condensed 600 label), quiet (transparent, 1px line border), destructive (rust text on quiet). height 32 desktop, 44 minimum on touch
- pips: n boxes 10 by 10px, filled boxes gold, count text beside
- placement numeral: condensed 700 28px, the shared tier split: gold when 1, chalk 2 to 4, slate 5 to 8
- rarity dot: 8px circle in the cost color, always adjacent to the cost digit and hero name
- star pips: 3 marks 6px plus the text "n/3", filled count is star level
- chips: panel fill, 1px line border, 3px radius, 13px text, each with a remove x mini form. item chips and relic chips look identical, the row label distinguishes them
- tables: condensed 600 13px slate headers, 2px bottom rule in line color, 40px rows, right aligned numeric cells, row hover lifts background to panel
- bars: the finishes cell described in the dashboard section, aria-label carries the 8 shares
- disclosures: native `details` elements for slot operations, relic add, lineup edit, and codex create forms
- busy: every mutating control carries `hx-disable="this"` and its form carries `hx-sync`, the dim rides the htmx request classes
- empty states: one sentence naming the next action, plus the primary button when a route exists
- field errors: inline under the field, rust text, naming the field and the fix. form rerenders preserve entered values

## motion

one settle animation: a rerendered region fades in and drops 4px over 160ms during the htmx settle phase. everything else is instant. `prefers-reduced-motion: reduce` disables the settle and every transition.

## copy

rules: verbs that say exactly what happens, the same word for the same action everywhere, sentence case, digits for numbers, no filler. empty states direct to the next action. errors never apologize and never stay vague.

fixed labels:

| action | label | confirm text when destructive |
|---|---|---|
| create match shell | new match, create match | |
| add lineup card | add lineup | |
| duplicate card | copy | |
| add hero slot | add hero | |
| save stars | save stars | |
| edit lineup scalars | edit, save lineup | |
| attach item | add item | |
| attach relic | add relic | |
| remove item or relic | x on the chip, no confirm, easily re-added | |
| save codex row | save hero, save item, save relic, save patch, save pro, save tier | |
| finalize | finalize match | |
| delete lineup | delete | delete this lineup? its heroes and items go with it. |
| delete slot | delete | remove this hero from the lineup? |
| delete codex row | delete hero, delete item, delete relic, delete patch, delete pro, delete race, delete class | delete grim jaw? existing matches keep their history. |

metric legend, shown on the dashboard:

- pick rate: share of lineups in view that played the hero
- top 4: of the lineups playing this hero, the share finishing 1st through 4th
- floor: the cautious reading of top 4, a wilson 95 percent lower bound, with the lineups on hand the true rate could be as low as this
- avg place: mean finishing placement of the lineups playing the hero, lower is better
- vs field: difference from the average placement of all lineups in the same filter, negative is better
- synergy lift: for a race or class at tier count k, average placement of lineups with at least k units minus the field average, negative is better
- item lift: average placement of slots holding the item minus the average placement of the same hero in slots without it, negative is better
- relic lift: same shape over the lineups holding the relic

empty states:

- codex: "no heroes yet. add the first hero so lineups can reference it."
- matches: "no matches yet. start one from the game you just finished."
- dashboard: "no finalized matches for this filter yet. finalize a few matches first."

error examples:

- "placement 4 is already used by another lineup in this match."
- "a lineup holds at most 12 heroes."
- "a slot holds at most 6 items."
- "no hero named x in the codex. add it first."
- "a pro match needs 8 lineups with placements 1 through 8 before it can be finalized."

## accessibility floor

- visible focus everywhere: 2px gold outline, 2px offset, on `:focus-visible`
- focus is managed across swaps per the swap contract table, because a swap that eats the focused control otherwise strands keyboard users at the page top
- every control has a visible label, no placeholder standing in for one
- placement, rarity, and star level never rely on color alone: the numeral, cost digit, and "n/3" text always render, bars carry aria-labels
- contrast pairs and ratios listed in the tokens section
- reduced motion respected, see motion
- tables use `th scope`, forms use `label for`
- at 900px and below: editor and detail grids drop to 1 column, the scoreboard strip wraps to 2 rows of 4 and to 2 columns below 600px, the slot grid keeps 4 columns until 720px then drops to 2, tables scroll horizontally inside their panel, touch targets grow to 44px

## implementation notes

```
internal/ui/static/
  app.css                      one stylesheet, budget under 400 sloc
  htmx.min.js                  vendored 4.0.0
  fonts/
    barlow-400.woff2
    barlow-500.woff2
    barlow-condensed-600.woff2
    barlow-condensed-700.woff2
```

- tokens are css custom properties on `:root` (`--ink`, `--panel`, `--line`, `--chalk`, `--slate`, `--gold`, `--rust`, `--c1` through `--c5`, `--place-light`, `--place-dim`), components consume tokens only
- component classes named by role: `topbar`, `strip`, `lcard`, `slot`, `chip`, `pip`, `spread`, `legend`. no utility classes, no cascade deeper than 2 levels
- templ components mirror the component inventory one to one (Button, Pips, LineupCard, SlotCell, Chip, SpreadBar, MetricLegend), so the spec and the code stay in lockstep
- numbers are formatted in go before they reach templates, templates never compute
- the swap contract is the single wiring surface: one rule, the id list, the focus table. attribute names and the HX-Redirect header were checked against htmx 4 docs, confirm all of it including the oob after-swap event name against the vendored file once when wiring
- css is hand written and stays under the 400 sloc budget by keeping the component count low: if a new visual need appears, extend an existing component before adding one

## design self-check

rejected generic tells, kept here as the checklist for future ui work:

- cream background with serif display and terracotta accent: readme pins dark, wrong subject
- near black with a single acid green accent: gold accent plus the semantic rarity ladder instead
- the saas card kit, uniform rounded cards with soft shadows: flat bordered panels, hierarchy from borders
- broadsheet hairlines with zero radius everywhere: 3px radius where fingers click, heavier rules under table headers
- all caps tracked eyebrows, meta strings joined with middle dots, arrow suffixes on links, mono faces for small labels: all dropped
- fade and slide entrances on every section: one settle on rerendered regions only

this spec survived 2 independent adversarial review passes (usability and accessibility, htmx feasibility and voice) on 2026-09-23, plus a per finding verification round on the fixes. future ui changes re-run this list and a review pass before shipping.
