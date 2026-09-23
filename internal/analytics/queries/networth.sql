-- Net worth curve: average net worth per final placement over the view.

WITH scope AS (
    SELECT l.placement, l.networth
    FROM ac.lineups l
    JOIN ac.matches m ON m.id = l.match_id
    WHERE m.finalized_at > 0{{filters}}
)
SELECT placement, count(*) AS n, avg(networth) AS avg_networth
FROM scope
GROUP BY placement
ORDER BY placement
