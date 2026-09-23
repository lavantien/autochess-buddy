-- Field average placement and lineup count for the finalized lineups in view.
-- An empty view scans as n = 0 with avg 0.

WITH scope AS (
    SELECT l.id, l.placement
    FROM ac.lineups l
    JOIN ac.matches m ON m.id = l.match_id
    WHERE m.finalized_at > 0{{filters}}
)
SELECT COALESCE(avg(placement), 0) AS avg_place, count(*) AS n
FROM scope
