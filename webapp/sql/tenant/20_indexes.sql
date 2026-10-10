-- 既存DB・初期データ復元後のDB用。10_schema.sqlと同じ索引。
CREATE INDEX IF NOT EXISTS player_score_competition_row
  ON player_score (tenant_id, competition_id, row_num DESC);

CREATE INDEX IF NOT EXISTS player_score_competition_player_row
  ON player_score (tenant_id, competition_id, player_id, row_num DESC);
