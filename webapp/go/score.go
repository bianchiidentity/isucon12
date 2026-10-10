package isuports

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// replaceCompetitionScores replaces one competition's scores atomically.
// The caller validates the complete CSV and holds the tenant lock.
func replaceCompetitionScores(ctx context.Context, tenantDB *sqlx.DB, tenantID int64, competitionID string, rows []PlayerScoreRow) error {
	// Keep the connection until commit-error cleanup is complete. go-sqlite3
	// v1.14.13 only rolls back SQLITE_BUSY commit failures, while database/sql
	// considers the Tx done even for other commit errors.
	conn, err := tenantDB.Connx(ctx)
	if err != nil {
		return fmt.Errorf("error Connect player_score replacement: %w", err)
	}
	defer conn.Close()
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error Begin player_score replacement: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM player_score WHERE tenant_id = ? AND competition_id = ?", tenantID, competitionID); err != nil {
		return fmt.Errorf("error Delete player_score: tenantID=%d, competitionID=%s, %w", tenantID, competitionID, err)
	}
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO player_score (id, tenant_id, player_id, competition_id, score, row_num, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("error Prepare player_score: %w", err)
	}
	defer stmt.Close()
	for _, ps := range rows {
		if _, err := stmt.ExecContext(ctx, ps.ID, ps.TenantID, ps.PlayerID, ps.CompetitionID, ps.Score, ps.RowNum, ps.CreatedAt, ps.UpdatedAt); err != nil {
			return fmt.Errorf("error Insert player_score: id=%s, tenant_id=%d, playerID=%s, competitionID=%s, score=%d, rowNum=%d, createdAt=%d, updatedAt=%d, %w", ps.ID, ps.TenantID, ps.PlayerID, ps.CompetitionID, ps.Score, ps.RowNum, ps.CreatedAt, ps.UpdatedAt, err)
		}
	}
	if err := tx.Commit(); err != nil {
		if _, rollbackErr := conn.ExecContext(context.Background(), "ROLLBACK"); rollbackErr != nil {
			// The driver may already have rolled back. In either case, do not
			// return a connection with an uncertain transaction state to the pool.
			_ = conn.Raw(func(interface{}) error { return driver.ErrBadConn })
		}
		return fmt.Errorf("error Commit player_score replacement: %w", err)
	}
	return nil
}
