# frontend design

## brief

autochess companion is a localhost tool for one person: a player of auto chess (drodo) who enters codex data by hand, logs pro match lineups, and reads the meta off computed numbers. the frontend has exactly 2 jobs: make manual entry fast enough to keep up with a match queue, and make analytics scannable at a glance.

hard constraints from the architecture (readme layer 5):

- server rendered templ components, htmx 4 vendored, no client js beyond htmx
- one hand written dark stylesheet, tables and forms only, no css framework, no build step
- hero picker is a select with datalist, no per keystroke server calls
- incremental lineup entry, each lineup card is its own form
- deletes use hx-delete with hx-confirm

## direction

the design borrows the game's own vernacular. auto chess is an 8 player auto battler with a dark stone board, a gold economy, a cost 1 to 5 rarity ladder, and a post match scoreboard that ranks 8 players. so: the lineup editor is rendered as the game's end of match scoreboard, a horizontal strip of 8 rank ordered lineup cards with big placement numerals. placement is a real ordinal, the numerals are data, not decoration. every other screen stays quiet: dense tables on flat bordered panels.

principles:

- color carries game meaning only. gold is the single accent (economy, victory, 5 cost, primary actions). the rarity ladder is the only multicolor system and it marks hero identity and cost, nothing else
- tables are the design. the dashboard, the codex, and the match list are ranked tables, no chart library, bars are css widths inside cells
- entry speed beats polish. defaults match the common case: stars 2, the copy button duplicates the previous card, placement conflicts surface on the card that caused them
- one bold spend. the lobby strip is the memorable element, everything around it is disciplined

## tokens

### color

| token | hex | use |
|---|---|---|
| ink | #12161C | page background, dark blue slate like the board stone |
| panel | #1A212A | raised surfaces: cards, forms, table headers on hover |
| line | #2A3542 | 1px borders, table rules |
| chalk | #E9E4D6 | primary text, warm white like the white pieces |
| slate | #9AA4B0 | muted text, table headers, secondary labels |
| gold | #D9A441 | the accent: primary buttons, active nav, 1st place, 5 cost |
| rust | #C25B4C | destructive actions |

rarity ladder, semantic only, always paired with the cost digit:

| cost | hex |
|---|---|
| 1 | #9AA3AE |
| 2 | #6FA96B |
| 3 | #5B8FD9 |
| 4 | #9B6FD9 |
| 5 | #D9A441 (gold) |

computed contrast: chalk on ink 14.3:1, gold on ink 8.1:1, slate on panel 6.4:1, ink on gold 8.1:1. all pairs pass wcag aa, chalk and gold pairs pass aaa for large text.

placement color coding: 1st carries gold, placements 2 to 4 render chalk, 5 to 8 render slate. the numeral itself is always shown, color never carries the rank alone.

### type

two roles, one family, distinct through width:

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
patch [7.5 v]  source [pro v]          124 lineups in view

hero performance

hero           cost  pick    top 4   floor   avg   vs field  finishes
sky breaker     5    41.2%   63.4%  55.1%   3.8    -0.6     [#::::::.]
grim jaw        4    38.7%   60.2%  53.8%   4.0    -0.4     [##:::::.]
lord of sand    2    21.0%   48.9%  44.0%   4.5    +0.1     [#::::::.]
```

- filter row is a plain form, submit reloads the page, filters ride query params (`/dashboard/heroes?patch=7.5&source=pro`)
- the finishes cell is a 72 by 10px bar of 8 segments, one per placement, segment width is the share of finishes at that placement, segment 1 gold, 2 to 4 slate at 80 percent, 5 to 8 slate at 40 percent
- a metric legend sits under the section title, always visible, plain words (see copy)
- synergy, item, and relic tables repeat this shape: name, sample size, lift, finishes bar
- empty state when a filter has no finalized matches: "no finalized matches for this filter yet. finalize a few matches first."

### matches list

```
matches                                                    [new match]

state   played             patch  source  lineups         notes
draft   2026-09-22 20:14   7.5    pro     [###.....] 3/8  vic lobby
final   2026-09-21 21:02   7.5    me      [#]        1/1  ranked grind
final   2026-09-20 22:40   7.4    pro     [########] 8/8  ghost cup
```

- the pips component shows one box per required lineup, 8 for pro matches, 1 for my matches, filled per entered lineup, with the n/8 count as text next to it
- draft rows link to the editor, final rows link to the match detail
- new match goes to a small form: patch select, source select (me, pro), played at datetime local, notes, then [create match]

### match editor, the lobby strip

```
edit match  patch 7.5  source pro       [###.....] 3/8  [finalize match]

+--------+ +--------+ +--------+ +--------------------+
|      1 | |      2 | |      3 | | add lineup        |
| vic    | | ghost  | | huan   | | placement [   4 ] |
| 7-2-1  | | 6-3-1  | | 5-4-2  | | pro       [ v   ] |
| nw 61  | | nw 58  | | nw 49  | | label     [     ] |
| board  | | board  | | board  | | [add lineup]      |
+--------+ +--------+ +--------+ +--------------------+
```

- cards sit in placement order, the strip scrolls horizontally when it overflows
- the add card is a permanent last card holding one small form: placement number, pro select or label, [add lineup]
- each lineup card carries a copy button that posts to the same add route with `copy_from` set, the server clones that card's heroes, stars, items, and relics, placement increments to the first free slot. adjacent placements share most pieces, this is the fastest path through 8 cards
- the header pips and n/8 count update after every add and delete
- finalize is a plain form post, no htmx: it is the terminal action, the server answers 303 to the match detail, the browser navigates. on rule failure the editor rerenders with the error banner naming the missing lineups or the duplicate placement

### lineup card anatomy

```
+-------------------------------+
|                           1   |   placement numeral, condensed 700
| vic  peak: king               |   pro handle or label, slate
| w-d-l  7-2-1       nw 61      |   numbers right aligned
| relics  [cursed blade] [ + ]  |   chips plus add disclosure
| +-------------+-------------+ |
| | o grim jaw      ( * * - ) | |   rarity dot, name, star pips
| |   void stone, fist        | |   item chips
| +-------------+-------------+ |
| | o dusk ranger    ( - - - )| |
| |   + item                  | |   per slot disclosure
| +-------------+-------------+ |
|   grid is 4 x 3, 12 slots    |
| +-------------+-------------+ |
| | add hero: [grim j..] *2v*  | |   datalist search, stars default 2
| +-------------+-------------+ |
| [copy]            [delete]    |
+-------------------------------+
```

- the slot grid is 4 columns by 3 rows, one slot per hero, empty slots render a quiet add hero control in place
- star pips are 3 small marks, filled count is the star level, always next to the hero name
- item attach is a native `details` disclosure inside the slot cell: item select, [add item]. no js
- the relics row holds relic chips and one `details` disclosure with a relic select, [add relic]
- the add hero row pins to the card bottom: datalist input filtered by the browser, stars select defaulting to 2, [add hero]
- copy and delete sit in the card footer, delete asks first (see copy)

### codex

```
heroes                                                    [+ new hero]

name          cost  races        classes     lineups  avg place
grim jaw       4    human        warrior       38      4.0
dusk ranger    3    elf, beast   hunter        21      4.4
```

- `+ new hero` opens a `details` create form above the table: name, cost select 1 to 5, race 1 plus optional race 2, class 1 plus optional class 2, ability, notes, [save hero]
- each row links to an edit page (`/heroes/{id}`) with the same form prefilled plus [delete hero]
- items, relics, patches, pros repeat the pattern with their own fields
- synergies shows two columns, races and classes, each entry is a tier ladder:

```
races                        classes
human                        warrior
  2   all humans +10% atk     2   warriors +30 armor
  4   all humans +25% atk     4   warriors +70 armor
  6   all humans +45% atk     6   warriors +120 armor
```

- codex tables show read only analytics columns (lineups, avg place) so the codex doubles as a slow dashboard for entry sanity

### match detail

```
match 14  patch 7.5  pro  finalized 2026-09-22

(strip of 8 read only lineup cards, same anatomy, no forms)

notes  vic lobby, ghost went 8th rolling warriors
```

## htmx swap map

htmx 4 facts this relies on, verified 2026-09-23: error responses swap by default, `hx-swap-oob` still exists and the main swap runs before oob swaps, `hx-confirm` is still core, attribute inheritance is opt-in so every interaction attribute goes on the element itself, `hx-disabled-elt` was renamed to `hx-disable`.

| action | request | response | target | mechanism |
|---|---|---|---|---|
| add lineup | POST /matches/{id}/lineups | lineup card partial + updated pips | strip card container | `hx-post` with `hx-swap="beforeend"`, pips via `hx-swap-oob` |
| copy lineup | POST /matches/{id}/lineups + copy_from | same as add | same | same |
| add hero slot | POST /lineups/{id}/slots | slot cell partial | slot grid | `hx-swap="beforeend"` |
| add item | POST /slots/{id}/items | slot cell partial | the slot cell | `hx-swap="outerHTML"` |
| add relic | POST /lineups/{id}/relics | relic chip partial | relic row | `hx-swap="beforeend"` |
| delete slot | DELETE /slots/{id} | 200 empty body | slot cell | `hx-swap="delete"` |
| delete lineup | DELETE /lineups/{id} | empty + updated pips | the card | `hx-swap="delete"`, pips oob |
| validation error | same as the action | 422 field error partial | the form that submitted | htmx 4 swaps error responses by default |
| finalize | POST /matches/{id}/finalize | 303 redirect | none | plain form, full navigation |

non-hx requests to any route get the full page, per the readme http contract.

## components and states

- buttons: primary (gold fill, ink text, condensed 600 label), quiet (transparent, 1px line border), destructive (rust text on quiet). height 32 desktop, 44 minimum on touch
- pips: n boxes 10 by 10px, filled boxes gold, count text beside
- placement numeral: condensed 700 28px, gold when 1, chalk 2 to 4, slate 5 to 8
- rarity dot: 8px circle in the cost color, always adjacent to the cost digit and hero name
- star pips: 3 marks 6px, filled count is star level
- chips: panel fill, 1px line border, 3px radius, 13px text. item chips and relic chips look identical, the row label distinguishes them
- tables: condensed 600 13px slate headers, 2px bottom rule in line color, 40px rows, right aligned numeric cells, row hover lifts background to panel
- bars: the finishes cell described in the dashboard section
- busy: buttons dim and disable while their request is in flight, driven by the htmx request classes, no extra attributes to maintain
- empty states: one sentence naming the next action, plus the primary button when a route exists
- field errors: inline under the field, rust text, naming the field and the fix. form rerenders preserve entered values

## motion

one settle animation: a partial appended by htmx fades in and drops 4px over 160ms during the htmx settle phase. everything else is instant. `prefers-reduced-motion: reduce` disables the settle and every transition.

## copy

rules: verbs that say exactly what happens, the same word for the same action everywhere, sentence case, digits for numbers, no filler. empty states direct to the next action. errors never apologize and never stay vague.

fixed labels:

| action | label | confirm text when destructive |
|---|---|---|
| create match shell | new match, create match | |
| add lineup card | add lineup | |
| duplicate card | copy | |
| add hero slot | add hero | |
| attach item | add item | |
| attach relic | add relic | |
| save codex row | save hero, save item, save relic, save patch, save pro | |
| finalize | finalize match | |
| delete lineup | delete | delete this lineup? its heroes and items go with it. |
| delete slot | delete | remove this hero from the lineup? |
| delete codex row | delete hero, delete item, delete relic | delete grim jaw? matches keep their history |

metric legend, shown on the dashboard:

- pick rate: share of lineups in view that played the hero
- top 4: share of lineups finishing 1st through 4th
- floor: the cautious reading of top 4, with the lineups on hand the true rate could be as low as this
- avg place: mean finishing placement, lower is better
- vs field: difference from the average placement of all lineups in the same filter, negative is better

empty states:

- codex: "no heroes yet. add the first hero so lineups can reference it."
- matches: "no matches yet. start one from the game you just finished."
- dashboard: "no finalized matches for this filter yet. finalize a few matches first."

error examples:

- "placement 4 is already used by another lineup in this match."
- "a lineup holds at most 12 heroes."
- "a slot holds at most 6 items."
- "a pro match needs 8 lineups with placements 1 through 8 before it can be finalized."

## accessibility floor

- visible focus everywhere: 2px gold outline, 2px offset, on `:focus-visible`
- every control has a visible label, no placeholder standing in for one
- placement and rarity never rely on color alone: the numeral and cost digit always render
- contrast pairs and ratios listed in the tokens section
- reduced motion respected, see motion
- tables use `th scope`, forms use `label for`
- at 720px and below: the strip stacks vertically, the slot grid drops to 2 columns, tables scroll horizontally inside their panel, touch targets grow to 44px

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

- tokens are css custom properties on `:root` (`--ink`, `--panel`, `--line`, `--chalk`, `--slate`, `--gold`, `--rust`, `--c1` through `--c5`), components consume tokens only
- component classes named by role: `topbar`, `strip`, `lcard`, `slot`, `chip`, `pip`, `spread`, `legend`. no utility classes, no cascade deeper than 2 levels
- templ components mirror the component inventory one to one (Button, Pips, LineupCard, SlotCell, Chip, SpreadBar, MetricLegend), so the spec and the code stay in lockstep
- numbers are formatted in go before they reach templates, templates never compute
- attribute names in the swap map were checked against htmx 4 docs, confirm against the vendored file once when wiring node C
- css is hand written and stays under the 400 sloc budget by keeping the component count low: if a new visual need appears, extend an existing component before adding one

## design self-check

rejected generic tells, kept here as the checklist for future ui work:

- cream background with serif display and terracotta accent: readme pins dark, wrong subject
- near black with a single acid green accent: gold accent plus the semantic rarity ladder instead
- the saas card kit, uniform rounded cards with soft shadows: flat bordered panels, hierarchy from borders
- broadsheet hairlines with zero radius everywhere: 3px radius where fingers click, heavier rules under table headers
- all caps tracked eyebrows, meta strings joined with middle dots, arrow suffixes on links, mono faces for small labels: all dropped
- fade and slide entrances on every section: one settle on appended cards only

future ui changes re-run this list before shipping.
