package isuports

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

type rankingScoreRow struct {
	PlayerScoreRow
	JoinedPlayerID    sql.NullString `db:"joined_player_id"`
	PlayerDisplayName sql.NullString `db:"player_display_name"`
}

// retrieveCompetitionRanks runs under the caller's tenant lock.
func retrieveCompetitionRanks(ctx context.Context, tenantDB dbOrTx, tenantID int64, competitionID string, rankAfter int64) ([]CompetitionRank, error) {
	pss := []rankingScoreRow{}
	if err := tenantDB.SelectContext(
		ctx,
		&pss,
		`SELECT ps.*, p.id AS joined_player_id, p.display_name AS player_display_name
        FROM player_score AS ps
        LEFT JOIN player AS p ON p.id = ps.player_id
        WHERE ps.tenant_id = ? AND ps.competition_id = ?
        ORDER BY ps.row_num DESC`,
		tenantID,
		competitionID,
	); err != nil {
		return nil, fmt.Errorf("error Select player_score: tenantID=%d, competitionID=%s, %w", tenantID, competitionID, err)
	}
	ranks := make([]CompetitionRank, 0, len(pss))
	scoredPlayerSet := make(map[string]struct{}, len(pss))
	for _, ps := range pss {
		// player_scoreが同一player_id内ではrow_numの降順でソートされているので
		// 現れたのが2回目以降のplayer_idはより大きいrow_numでスコアが出ているとみなせる
		if _, ok := scoredPlayerSet[ps.PlayerID]; ok {
			continue
		}
		scoredPlayerSet[ps.PlayerID] = struct{}{}
		// LEFT JOIN keeps broken references visible instead of silently dropping scores.
		if !ps.JoinedPlayerID.Valid {
			return nil, fmt.Errorf("error retrievePlayer: error Select player: id=%s, %w", ps.PlayerID, sql.ErrNoRows)
		}
		ranks = append(ranks, CompetitionRank{
			Score:             ps.Score,
			PlayerID:          ps.JoinedPlayerID.String,
			PlayerDisplayName: ps.PlayerDisplayName.String,
			RowNum:            ps.RowNum,
		})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].Score == ranks[j].Score {
			return ranks[i].RowNum < ranks[j].RowNum
		}
		return ranks[i].Score > ranks[j].Score
	})
	pagedRanks := make([]CompetitionRank, 0, 100)
	for i, rank := range ranks {
		if int64(i) < rankAfter {
			continue
		}
		pagedRanks = append(pagedRanks, CompetitionRank{
			Rank:              int64(i + 1),
			Score:             rank.Score,
			PlayerID:          rank.PlayerID,
			PlayerDisplayName: rank.PlayerDisplayName,
		})
		if len(pagedRanks) >= 100 {
			break
		}
	}

	return pagedRanks, nil
}
