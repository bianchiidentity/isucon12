-- Run after 20_indexes.sql. Keep the original CSV row numbers (tie-break order).
-- Partition by tenant and competition as well as player. Safe to rerun.
DELETE FROM player_score
WHERE row_num < (
  SELECT MAX(latest.row_num)
  FROM player_score AS latest
  WHERE latest.tenant_id = player_score.tenant_id
    AND latest.competition_id = player_score.competition_id
    AND latest.player_id = player_score.player_id
);
