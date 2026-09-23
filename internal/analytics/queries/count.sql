-- Lineup count for the finalized lineups in view.

WITH scope AS (
    SELECT l.id
    FROM ac.lineups l
    JOIN ac.matches m ON m.id = l.match_id
    WHERE m.finalized_at > 0{{filters}}
)
SELECT count(*) AS n
FROM scope
