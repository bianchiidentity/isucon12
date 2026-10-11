package isuports

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// The caller has validated the header and holds the tenant lock. Every input
// row is validated, but only each player's last row is assigned an ID and saved.
func readCompetitionScores(ctx context.Context, db dbOrTx, tenantID int64, competitionID string, r *csv.Reader) ([]PlayerScoreRow, int64, error) {
	rows := []PlayerScoreRow{}
	positions := map[string]int{}
	var count int64
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("error r.Read at rows: %w", err)
		}
		count++
		if len(row) != 2 {
			return nil, 0, fmt.Errorf("row must have two columns: %#v", row)
		}
		playerID, scoreStr := row[0], row[1]
		position, seen := positions[playerID]
		if !seen {
			if _, err := retrievePlayer(ctx, db, playerID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, 0, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("player not found: %s", playerID))
				}
				return nil, 0, fmt.Errorf("error retrievePlayer: %w", err)
			}
		}
		score, err := strconv.ParseInt(scoreStr, 10, 64)
		if err != nil {
			return nil, 0, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error strconv.ParseUint: scoreStr=%s, %s", scoreStr, err))
		}
		now := time.Now().Unix()
		ps := PlayerScoreRow{
			TenantID: tenantID, CompetitionID: competitionID, PlayerID: playerID,
			Score: score, RowNum: count, CreatedAt: now, UpdatedAt: now,
		}
		if seen {
			rows[position] = ps
		} else {
			positions[playerID] = len(rows)
			rows = append(rows, ps)
		}
	}
	for i := range rows {
		id, err := dispenseID(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("error dispenseID: %w", err)
		}
		rows[i].ID = id
	}
	return rows, count, nil
}
