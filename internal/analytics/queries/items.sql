-- Item lift: average placement of view slots holding the item minus average
-- placement of slots of the same heroes without it (readme). Items never held
-- in view do not appear. held and baseline aggregate separately because a
-- direct join would cross the slot sets and corrupt the averages.

WITH scope AS (
    SELECT l.id, l.placement
    FROM ac.lineups l
    JOIN ac.matches m ON m.id = l.match_id
    WHERE m.finalized_at > 0{{filters}}
),
slots AS (
    SELECT v.id AS lineup_id, v.placement, s.id AS slot_id, s.hero_id
    FROM scope v
    JOIN ac.lineup_slots s ON s.lineup_id = v.id
),
held AS (
    SELECT sl.placement, sl.hero_id, si.item_id
    FROM slots sl
    JOIN ac.slot_items si ON si.slot_id = sl.slot_id
),
baseline AS (
    SELECT h.item_id, b.placement
    FROM held h
    JOIN slots b ON b.hero_id = h.hero_id
    WHERE NOT EXISTS (
        SELECT 1 FROM ac.slot_items si
        WHERE si.slot_id = b.slot_id AND si.item_id = h.item_id
    )
),
held_agg AS (
    SELECT item_id, count(*) AS slots_with, avg(placement) AS avg_place,
        count(*) FILTER (WHERE placement = 1) AS p1,
        count(*) FILTER (WHERE placement = 2) AS p2,
        count(*) FILTER (WHERE placement = 3) AS p3,
        count(*) FILTER (WHERE placement = 4) AS p4,
        count(*) FILTER (WHERE placement = 5) AS p5,
        count(*) FILTER (WHERE placement = 6) AS p6,
        count(*) FILTER (WHERE placement = 7) AS p7,
        count(*) FILTER (WHERE placement = 8) AS p8
    FROM held
    GROUP BY item_id
),
base_agg AS (
    SELECT item_id, count(*) AS n, avg(placement) AS avg_place
    FROM baseline
    GROUP BY item_id
),
view_count AS (
    SELECT count(*) AS n FROM scope
)
SELECT
    i.id,
    i.name,
    i.tier,
    i.effect,
    ha.slots_with AS slots_with,
    (SELECT n FROM view_count) AS lineups_in_view,
    CASE WHEN b.n IS NULL OR b.n = 0 THEN 0 ELSE ha.avg_place - b.avg_place END AS lift,
    ha.p1, ha.p2, ha.p3, ha.p4, ha.p5, ha.p6, ha.p7, ha.p8
FROM ac.items i
JOIN held_agg ha ON ha.item_id = i.id
LEFT JOIN base_agg b ON b.item_id = i.id
ORDER BY i.id
