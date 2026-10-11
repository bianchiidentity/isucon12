-- 既存DBへ1回だけ適用する。接続時に対象データベースを指定する。
-- 新規構築ではadmin/10_schema.sqlで同じ索引を作成済みのため適用不要。
-- /initializeはDELETEで履歴を削除するため、この索引は維持される。
ALTER TABLE visit_history
  ADD INDEX visit_history_billing (tenant_id, competition_id, player_id, created_at),
  ALGORITHM=INPLACE, LOCK=NONE;
