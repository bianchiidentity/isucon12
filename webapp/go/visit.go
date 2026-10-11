package isuports

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
)

// The unique key serializes concurrent visits. The earliest timestamp wins even
// when requests reach MySQL in the opposite order. Later visits are no-op updates.
const recordFirstVisitQuery = `INSERT INTO visit_history
 (player_id, tenant_id, competition_id, created_at, updated_at)
 VALUES (?, ?, ?, ?, ?)
 ON DUPLICATE KEY UPDATE
 created_at = LEAST(created_at, VALUES(created_at)),
 updated_at = LEAST(updated_at, VALUES(updated_at))`

func recordFirstVisit(ctx context.Context, db *sqlx.DB, tenantID int64, competitionID, playerID string, at int64) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, recordFirstVisitQuery, playerID, tenantID, competitionID, at, at)
	if err != nil {
		return fmt.Errorf("error Upsert visit_history: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	// Revision and earliest visit commit atomically. A delayed first visit
	// invalidates any report computed before that visit became visible.
	if changed != 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO billing_versions (tenant_id, competition_id, version) VALUES (?, ?, 1)
   ON DUPLICATE KEY UPDATE version = version + 1`, tenantID, competitionID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
