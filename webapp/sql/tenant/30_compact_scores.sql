-- Compute latest rows once, then compact before building runtime indexes.
-- Preserve original CSV row numbers and all ties at the maximum row_num.
BEGIN;
CREATE TEMP TABLE compact_score_latest AS
 SELECT tenant_id, competition_id, player_id, MAX(row_num) AS row_num
 FROM player_score GROUP BY tenant_id, competition_id, player_id;
CREATE UNIQUE INDEX compact_score_latest_key
 ON compact_score_latest (tenant_id, competition_id, player_id);
DELETE FROM player_score WHERE row_num < (
 SELECT latest.row_num FROM compact_score_latest AS latest
 WHERE latest.tenant_id = player_score.tenant_id
 AND latest.competition_id = player_score.competition_id
 AND latest.player_id = player_score.player_id
);
DROP TABLE compact_score_latest;
COMMIT;
