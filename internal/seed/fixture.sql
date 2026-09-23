-- Shared seed fixture (readme:262): one sql script feeds dev seeding, the analytics
-- goldens and the e2e fakes. All ids are explicit so every row is addressable by tests.
--
-- Invariants the fixture builds in (frontend-design:110): the finalized pro view holds
-- 4 matches x 8 placements = 32 lineups, divisible by 8, and every match averages
-- (1+..+8)/8 = 4.5, so the field average placement is exactly 4.5.
--
-- Placement multisets per hero over the finalized pro view, hand-derived:
--   a lineup at placement p of match m draws the hero group g = ((p + m - 2) % 4) + 1,
--   expressed for lineup id L as (((L-1) % 8) + ((L-1) / 8)) % 4 + 1 below.
--   The four groups sit at placements
--     m1: 1,2,3,4,1,2,3,4   m2: 2,3,4,1,2,3,4,1
--     m3: 3,4,1,2,3,4,1,2   m4: 4,1,2,3,4,1,2,3
--   so each group covers {1,5} + {4,8} + {3,7} + {2,6} = placements 1..8 once each.
--   Every hero h1..h12 therefore has the placement multiset {1,2,3,4,5,6,7,8}:
--   n = 8, top4 = 4, avg place = 4.5 (grim jaw, h2, is the TestSeed_GoldenSpot).
--
-- Further facts for analytics goldens:
--   items: finalized slot s in 1..96 carries item ((s-1) % 8) + 1, so each of the
--     8 items sits on 12 slots in the finalized pro view.
--   relics: finalized lineup L in 1..32 carries relic ((L-1) % 4) + 1, so each of the
--     4 relics sits on 8 lineups.
--   networth by placement: placement p carries wins 8-p, losses p-1 and networth
--     1000 + (8-p)*50 in all 4 pro matches, so avg networth per placement is that
--     constant with n = 4.

INSERT INTO patches (id, version, released_at) VALUES
  (1, '7.4', '2026-08-01'),
  (2, '7.5', '2026-09-01');

INSERT INTO races (id, name) VALUES
  (1, 'warrior'), (2, 'mage'), (3, 'beast');
INSERT INTO race_tiers (race_id, count, effect) VALUES
  (1, 2, '+20 armor'), (1, 4, '+45 armor'), (1, 6, '+90 armor'),
  (2, 2, '+15 spell power'), (2, 4, '+40 spell power'), (2, 6, '+80 spell power'),
  (3, 2, '+150 max hp'), (3, 4, '+400 max hp'), (3, 6, '+900 max hp');

INSERT INTO classes (id, name) VALUES
  (1, 'knight'), (2, 'assassin'), (3, 'druid');
INSERT INTO class_tiers (class_id, count, effect) VALUES
  (1, 2, 'block one spell'), (1, 4, 'block two spells'), (1, 6, 'block all spells'),
  (2, 2, 'crit 20 percent'), (2, 4, 'crit 45 percent'), (2, 6, 'crit 80 percent'),
  (3, 2, 'regen 2 percent'), (3, 4, 'regen 5 percent'), (3, 6, 'regen 10 percent');

-- groups A..D are consecutive triples: group g holds heroes 3g-2..3g. Costs cover 1-5.
-- h8 carries two races (mage, beast) and h9 two classes (assassin, druid): the
-- 2 dual-lineage pieces. h1-h3 reuse the frontend-design sample names.
INSERT INTO heroes (id, name, cost) VALUES
  (1, 'sky breaker', 5), (2, 'grim jaw', 4), (3, 'lord of sand', 2),
  (4, 'iron warden', 1), (5, 'frost weaver', 3), (6, 'night fang', 3),
  (7, 'ember cub', 1), (8, 'storm herald', 5), (9, 'veil dancer', 2),
  (10, 'anvil monk', 4), (11, 'moss tender', 1), (12, 'gale piercer', 5);
INSERT INTO hero_races (hero_id, race_id) VALUES
  (1, 1), (2, 1), (3, 2), (4, 1), (5, 2), (6, 3), (7, 3),
  (8, 2), (8, 3), (9, 3), (10, 2), (11, 3), (12, 2);
INSERT INTO hero_classes (hero_id, class_id) VALUES
  (1, 1), (2, 2), (3, 3), (4, 1), (5, 3), (6, 2), (7, 1), (8, 1),
  (9, 2), (9, 3), (10, 1), (11, 3), (12, 2);

INSERT INTO items (id, name, tier) VALUES
  (1, 'rusty blade', 1), (2, 'iron plate', 1), (3, 'swift boots', 2), (4, 'blood charm', 2),
  (5, 'storm core', 3), (6, 'titan belt', 3), (7, 'void scepter', 4), (8, 'phoenix feather', 4);
-- 2 recipes, one row per component: void scepter = storm core + titan belt,
-- phoenix feather = swift boots + blood charm.
INSERT INTO item_recipes (result_id, component_id) VALUES
  (7, 5), (7, 6), (8, 3), (8, 4);

INSERT INTO relics (id, name) VALUES
  (1, 'crown of ash'), (2, 'tide bell'), (3, 'hollow idol'), (4, 'sun engine');

INSERT INTO pros (id, name, handle, peak_rank) VALUES
  (1, 'nova', 'nova', 'challenger'), (2, 'kestrel', 'kestrel', 'grandmaster'),
  (3, 'moth', 'moth', 'grandmaster'), (4, 'atlas', 'atlas', 'master');

-- finalized pro matches m1..m4 (2 per patch), the me match m5, draft m6 (finalized_at 0).
INSERT INTO matches (id, patch_id, played_at, source, created_at, finalized_at) VALUES
  (1, 1, 1787000000, 'pro', 1787000000, 1787003600),
  (2, 1, 1787010000, 'pro', 1787010000, 1787013600),
  (3, 2, 1787100000, 'pro', 1787100000, 1787103600),
  (4, 2, 1787110000, 'pro', 1787110000, 1787113600),
  (5, 2, 1787200000, 'me', 1787200000, 1787203600),
  (6, 2, 1787300000, 'pro', 1787300000, 0);

-- 32 finalized pro lineups: placements 1..8 per match, pros cycle nova..atlas,
-- wins = 8-p, losses = p-1, networth = 1000 + wins*50.
WITH RECURSIVE seq(p) AS (SELECT 1 UNION ALL SELECT p + 1 FROM seq WHERE p < 8)
INSERT INTO lineups (id, match_id, pro_id, label, placement, wins, draws, losses, networth, created_at)
SELECT (m.id - 1) * 8 + s.p, m.id, ((s.p - 1) % 4) + 1,
       (SELECT handle FROM pros WHERE pros.id = ((s.p - 1) % 4) + 1) || ' board',
       s.p, 8 - s.p, 0, s.p - 1, 1000 + (8 - s.p) * 50, m.played_at
FROM matches m CROSS JOIN seq s
WHERE m.id <= 4;

-- irregular tail: the me lineup and the 3 draft lineups (the edit/delete e2e substrate).
INSERT INTO lineups (id, match_id, pro_id, label, placement, wins, draws, losses, networth, created_at) VALUES
  (33, 5, NULL, 'my board', 1, 5, 1, 2, 1200, 1787200000),
  (34, 6, 1, 'draft board a', 1, 0, 0, 0, 0, 1787300000),
  (35, 6, 1, 'draft board b', 2, 0, 0, 0, 0, 1787300000),
  (36, 6, 1, 'draft board c', 3, 0, 0, 0, 0, 1787300000);

-- finalized boards: the 3 consecutive heroes of group g, all 2 stars.
INSERT INTO lineup_slots (id, lineup_id, hero_id, slot_index, stars)
SELECT (l.id - 1) * 3 + s.k + 1, l.id,
       3 * (((l.id - 1) % 8 + (l.id - 1) / 8) % 4) + s.k + 1, s.k, 2
FROM lineups l CROSS JOIN (SELECT 0 AS k UNION ALL SELECT 1 UNION ALL SELECT 2) s
WHERE l.id <= 32;

-- me and draft boards, literal: stars cover 1..3.
INSERT INTO lineup_slots (id, lineup_id, hero_id, slot_index, stars) VALUES
  (97, 33, 1, 0, 2), (98, 33, 5, 1, 2), (99, 33, 9, 2, 3),
  (100, 34, 1, 0, 3), (101, 34, 2, 1, 2), (102, 34, 3, 2, 2),
  (103, 34, 4, 3, 2), (104, 34, 5, 4, 1), (105, 34, 6, 5, 1),
  (106, 35, 7, 0, 2), (107, 35, 8, 1, 2),
  (108, 36, 9, 0, 2), (109, 36, 10, 1, 3);

INSERT INTO slot_items (slot_id, item_id)
SELECT id, ((id - 1) % 8) + 1 FROM lineup_slots WHERE id <= 96;
INSERT INTO slot_items (slot_id, item_id) VALUES
  (97, 5), (98, 3), (99, 8),
  (100, 1), (100, 2), (101, 3), (102, 4), (103, 5), (104, 6), (105, 7),
  (106, 8), (107, 1), (108, 2), (109, 3);

INSERT INTO lineup_relics (lineup_id, relic_id)
SELECT id, ((id - 1) % 4) + 1 FROM lineups WHERE id <= 32;
INSERT INTO lineup_relics (lineup_id, relic_id) VALUES
  (33, 4), (34, 1), (34, 2), (35, 3);
