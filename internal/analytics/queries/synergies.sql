-- Synergy lift per race and class tier: average placement of view lineups
-- fielding at least the tier count minus the field average, negative is better
-- (readme). A slice with no lineups falls back to the field average, so its
-- lift is exactly 0. The units CTE unions the race and class halves so one
-- counts relation feeds both tier ladders.

WITH scope AS (
    SELECT l.id, l.placement
    FROM ac.lineups l
    JOIN ac.matches m ON m.id = l.match_id
    WHERE m.finalized_at > 0{{filters}}
),
units AS (
    SELECT s.lineup_id, v.placement, 'race' AS kind, hr.race_id AS lineage_id
    FROM scope v
    JOIN ac.lineup_slots s ON s.lineup_id = v.id
    JOIN ac.hero_races hr ON hr.hero_id = s.hero_id
    UNION ALL
    SELECT s.lineup_id, v.placement, 'class', hc.class_id
    FROM scope v
    JOIN ac.lineup_slots s ON s.lineup_id = v.id
    JOIN ac.hero_classes hc ON hc.hero_id = s.hero_id
),
counts AS (
    SELECT kind, lineage_id, lineup_id, placement, count(*) AS unit_count
    FROM units
    GROUP BY kind, lineage_id, lineup_id, placement
),
field AS (
    SELECT avg(placement) AS avg_place
    FROM scope
),
race_rows AS (
    SELECT
        'race' AS kind,
        t.race_id AS lineage_id,
        r.name AS name,
        t."count" AS tier_count,
        count(c.lineup_id) AS lineups,
        COALESCE(avg(c.placement), f.avg_place, 0) - COALESCE(f.avg_place, 0) AS lift,
        count(c.lineup_id) FILTER (WHERE c.placement = 1) AS p1,
        count(c.lineup_id) FILTER (WHERE c.placement = 2) AS p2,
        count(c.lineup_id) FILTER (WHERE c.placement = 3) AS p3,
        count(c.lineup_id) FILTER (WHERE c.placement = 4) AS p4,
        count(c.lineup_id) FILTER (WHERE c.placement = 5) AS p5,
        count(c.lineup_id) FILTER (WHERE c.placement = 6) AS p6,
        count(c.lineup_id) FILTER (WHERE c.placement = 7) AS p7,
        count(c.lineup_id) FILTER (WHERE c.placement = 8) AS p8
    FROM ac.race_tiers t
    JOIN ac.races r ON r.id = t.race_id
    CROSS JOIN field f
    LEFT JOIN counts c
        ON c.kind = 'race' AND c.lineage_id = t.race_id AND c.unit_count >= t."count"
    GROUP BY t.race_id, r.name, t."count", f.avg_place
),
class_rows AS (
    SELECT
        'class' AS kind,
        t.class_id AS lineage_id,
        cl.name AS name,
        t."count" AS tier_count,
        count(c.lineup_id) AS lineups,
        COALESCE(avg(c.placement), f.avg_place, 0) - COALESCE(f.avg_place, 0) AS lift,
        count(c.lineup_id) FILTER (WHERE c.placement = 1) AS p1,
        count(c.lineup_id) FILTER (WHERE c.placement = 2) AS p2,
        count(c.lineup_id) FILTER (WHERE c.placement = 3) AS p3,
        count(c.lineup_id) FILTER (WHERE c.placement = 4) AS p4,
        count(c.lineup_id) FILTER (WHERE c.placement = 5) AS p5,
        count(c.lineup_id) FILTER (WHERE c.placement = 6) AS p6,
        count(c.lineup_id) FILTER (WHERE c.placement = 7) AS p7,
        count(c.lineup_id) FILTER (WHERE c.placement = 8) AS p8
    FROM ac.class_tiers t
    JOIN ac.classes cl ON cl.id = t.class_id
    CROSS JOIN field f
    LEFT JOIN counts c
        ON c.kind = 'class' AND c.lineage_id = t.class_id AND c.unit_count >= t."count"
    GROUP BY t.class_id, cl.name, t."count", f.avg_place
)
SELECT * FROM race_rows
UNION ALL
SELECT * FROM class_rows
ORDER BY kind, lineage_id, tier_count
