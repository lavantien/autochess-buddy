-- Hero pick performance over the finalized lineups in view. The guard clauses
-- for the optional patch and source filters expand from the filters token in
-- Go. Heroes with no pick in view drop out through the inner join.

WITH scope AS (
    SELECT l.id, l.placement
    FROM ac.lineups l
    JOIN ac.matches m ON m.id = l.match_id
    WHERE m.finalized_at > 0{{filters}}
),
picks AS (
    SELECT v.placement, s.hero_id
    FROM scope v
    JOIN ac.lineup_slots s ON s.lineup_id = v.id
),
field AS (
    SELECT count(*) AS n, avg(placement) AS avg_place
    FROM scope
)
SELECT
    h.id,
    h.name,
    h.cost,
    h.ability,
    h.notes,
    count(*) AS picks,
    count(*) FILTER (WHERE p.placement <= 4) AS top4,
    avg(p.placement) AS avg_place,
    count(*) FILTER (WHERE p.placement = 1) AS p1,
    count(*) FILTER (WHERE p.placement = 2) AS p2,
    count(*) FILTER (WHERE p.placement = 3) AS p3,
    count(*) FILTER (WHERE p.placement = 4) AS p4,
    count(*) FILTER (WHERE p.placement = 5) AS p5,
    count(*) FILTER (WHERE p.placement = 6) AS p6,
    count(*) FILTER (WHERE p.placement = 7) AS p7,
    count(*) FILTER (WHERE p.placement = 8) AS p8,
    (SELECT n FROM field) AS lineups_in_view,
    (SELECT avg_place FROM field) AS field_avg
FROM ac.heroes h
JOIN picks p ON p.hero_id = h.id
GROUP BY h.id, h.name, h.cost, h.ability, h.notes
ORDER BY h.id
