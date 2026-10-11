-- One-time migration. Stop the app and benchmark before running this file.
-- Do not run with mysql --force. Existing backup/staging tables deliberately fail.
-- Do not invoke from /initialize: the initial aggregation can be expensive.
CREATE TABLE visit_history_compacted LIKE visit_history;
ALTER TABLE visit_history_compacted
  ADD PRIMARY KEY (tenant_id, competition_id, player_id),
  ADD INDEX visit_history_created_at (created_at);

INSERT INTO visit_history_compacted
  (tenant_id, competition_id, player_id, created_at, updated_at)
SELECT tenant_id, competition_id, player_id, MIN(created_at), MIN(updated_at)
FROM visit_history
GROUP BY tenant_id, competition_id, player_id;

-- Atomic switch; retain the original history for rollback until validation passes.
RENAME TABLE visit_history TO visit_history_before_first_visit,
             visit_history_compacted TO visit_history;
