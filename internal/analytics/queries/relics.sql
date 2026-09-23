-- Relic lift: average placement of view lineups holding the relic minus
-- average placement of the view lineups without it (readme, same shape as
-- item lift). Relics never held in view do not appear. The without-half is
-- the field total minus the holding total; when every lineup in view holds
-- the relic there is no without-half and the lift falls back to 0.

WITH scope AS (
    SELECT l.id, l.placement
    FROM ac.lineups l
    JOIN ac.matches m ON m.id = l.match_id
    WHERE m.finalized_at > 0{{filters}}
),
holding AS (
    SELECT lr.relic_id, v.placement
    FROM scope v
    JOIN ac.lineup_relics lr ON lr.lineup_id = v.id
),
hold_agg AS (
    SELECT relic_id, count(*) AS lineups, sum(placement) AS sum_place,
        avg(placement) AS avg_place,
        count(*) FILTER (WHERE placement = 1) AS p1,
        count(*) FILTER (WHERE placement = 2) AS p2,
        count(*) FILTER (WHERE placement = 3) AS p3,
        count(*) FILTER (WHERE placement = 4) AS p4,
        count(*) FILTER (WHERE placement = 5) AS p5,
        count(*) FILTER (WHERE placement = 6) AS p6,
        count(*) FILTER (WHERE placement = 7) AS p7,
        count(*) FILTER (WHERE placement = 8) AS p8
    FROM holding
    GROUP BY relic_id
),
field AS (
    SELECT count(*) AS n, sum(placement) AS sum_place
    FROM scope
)
SELECT
    rl.id,
    rl.name,
    rl.effect,
    h.lineups AS lineups,
    f.n AS lineups_in_view,
    CASE WHEN f.n = h.lineups THEN 0
         ELSE h.avg_place - (f.sum_place - h.sum_place) / (f.n - h.lineups)
    END AS lift,
    h.p1, h.p2, h.p3, h.p4, h.p5, h.p6, h.p7, h.p8
FROM ac.relics rl
JOIN hold_agg h ON h.relic_id = rl.id
CROSS JOIN field f
ORDER BY rl.id
