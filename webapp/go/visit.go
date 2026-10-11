package isuports

import (
	"context"
	"fmt"
)

// The unique key serializes concurrent visits. The earliest timestamp wins even
// when requests reach MySQL in the opposite order. Later visits are no-op updates.
const recordFirstVisitQuery = `INSERT INTO visit_history
 (player_id, tenant_id, competition_id, created_at, updated_at)
 VALUES (?, ?, ?, ?, ?)
 ON DUPLICATE KEY UPDATE
 created_at = LEAST(created_at, VALUES(created_at)),
 updated_at = LEAST(updated_at, VALUES(updated_at))`

func recordFirstVisit(ctx context.Context, db dbOrTx, tenantID int64, competitionID, playerID string, at int64) error {
	if _, err := db.ExecContext(ctx, recordFirstVisitQuery, playerID, tenantID, competitionID, at, at); err != nil {
		return fmt.Errorf("error Upsert visit_history: playerID=%s, tenantID=%d, competitionID=%s, createdAt=%d, %w", playerID, tenantID, competitionID, at, err)
	}
	return nil
}
