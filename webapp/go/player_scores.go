package isuports

import (
	"context"
	"fmt"
)

// The correlated lookup uses the existing competition/player/row_num index
// to select one row per competition, without scanning all score history.
// rowid joins that exact row: the last CSV row wins, not the largest score.
const playerScoresQuery = `SELECT c.title AS competition_title, ps.score
FROM competition AS c
JOIN player_score AS ps ON ps.rowid = (
 SELECT rowid FROM player_score
 WHERE tenant_id = c.tenant_id AND competition_id = c.id AND player_id = ?
 ORDER BY row_num DESC LIMIT 1
)
WHERE c.tenant_id = ?
ORDER BY c.created_at ASC`

// retrievePlayerScores runs under the caller's tenant lock.
func retrievePlayerScores(ctx context.Context, db dbOrTx, tenantID int64, playerID string) ([]PlayerScoreDetail, error) {
	scores := []PlayerScoreDetail{}
	if err := db.SelectContext(ctx, &scores, playerScoresQuery, playerID, tenantID); err != nil {
		return nil, fmt.Errorf("error Select player scores: tenantID=%d, playerID=%s, %w", tenantID, playerID, err)
	}
	return scores, nil
}
