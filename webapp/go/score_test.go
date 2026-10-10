package isuports

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
)

func scoreRows(t *testing.T, db *sqlx.DB) []PlayerScoreRow {
	t.Helper()
	rows := []PlayerScoreRow{}
	if err := db.Select(&rows, "SELECT * FROM player_score ORDER BY id"); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestReplaceCompetitionScores(t *testing.T) {
	fixture := newRankingTestDB(t)
	db := fixture.DB
	addRankingPlayer(t, fixture, "a", "A", false)
	addRankingPlayer(t, fixture, "b", "B", false)
	addRankingScore(t, fixture, 1, "c", "a", 1, 999)
	addRankingScore(t, fixture, 1, "other", "b", 1, 88)
	addRankingScore(t, fixture, 2, "c", "b", 1, 77)
	rows := []PlayerScoreRow{
		{ID: "new1", TenantID: 1, PlayerID: "a", CompetitionID: "c", Score: 100, RowNum: 1, CreatedAt: 10, UpdatedAt: 11},
		{ID: "new2", TenantID: 1, PlayerID: "b", CompetitionID: "c", Score: 80, RowNum: 2, CreatedAt: 12, UpdatedAt: 13},
		{ID: "new3", TenantID: 1, PlayerID: "a", CompetitionID: "c", Score: 80, RowNum: 3, CreatedAt: 14, UpdatedAt: 15},
	}
	if err := replaceCompetitionScores(context.Background(), db, 1, "c", rows); err != nil {
		t.Fatal(err)
	}
	all := scoreRows(t, db)
	if len(all) != 5 || !reflect.DeepEqual(all[2:], rows) {
		t.Fatalf("replacement: %#v", all)
	}
	ranks, err := retrieveCompetitionRanks(context.Background(), db, 1, "c", 0)
	if err != nil || len(ranks) != 2 || ranks[0].PlayerID != "b" || ranks[1].Score != 80 {
		t.Fatalf("ranking: %#v %v", ranks, err)
	}
	// A header-only CSV clears only the target competition.
	if err := replaceCompetitionScores(context.Background(), db, 1, "c", nil); err != nil {
		t.Fatal(err)
	}
	if got := scoreRows(t, db); !reflect.DeepEqual(got, all[:2]) {
		t.Fatalf("empty replacement: %#v", got)
	}
}

func TestReplaceCompetitionScoresRollback(t *testing.T) {
	for _, phase := range []string{"Delete", "Insert", "Commit"} {
		t.Run(phase, func(t *testing.T) {
			fixture := newRankingTestDB(t)
			db := fixture.DB
			addRankingScore(t, fixture, 1, "c", "a", 1, 111)
			addRankingScore(t, fixture, 1, "other", "b", 2, 222)
			before := scoreRows(t, db)
			var setup string
			switch phase {
			case "Delete":
				setup = `CREATE TRIGGER fail_delete BEFORE DELETE ON player_score BEGIN SELECT RAISE(ABORT, 'delete failure'); END;`
			case "Insert":
				setup = `CREATE TRIGGER fail_insert BEFORE INSERT ON player_score WHEN NEW.id = 'bad' BEGIN SELECT RAISE(ABORT, 'insert failure'); END;`
			case "Commit":
				// Deferred constraints let every INSERT succeed and make COMMIT itself fail.
				setup = `PRAGMA foreign_keys = ON;
     CREATE TABLE parent (id TEXT PRIMARY KEY);
     CREATE TABLE guard (id TEXT REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED);
     CREATE TRIGGER fail_commit AFTER INSERT ON player_score WHEN NEW.id = 'bad' BEGIN INSERT INTO guard VALUES ('missing'); END;`
			}
			if _, err := db.Exec(setup); err != nil {
				t.Fatal(err)
			}
			rows := []PlayerScoreRow{
				{ID: "good", TenantID: 1, CompetitionID: "c", PlayerID: "a", Score: 20, RowNum: 1},
				{ID: "bad", TenantID: 1, CompetitionID: "c", PlayerID: "b", Score: 30, RowNum: 2},
			}
			err := replaceCompetitionScores(context.Background(), db, 1, "c", rows)
			if err == nil || !strings.Contains(err.Error(), "error "+phase) {
				t.Fatalf("%s: %v", phase, err)
			}
			if got := scoreRows(t, db); !reflect.DeepEqual(got, before) {
				t.Fatalf("partial update after %s: %#v", phase, got)
			}
			if _, err := db.Exec("DROP TRIGGER fail_" + strings.ToLower(phase)); err != nil {
				t.Fatal(err)
			}
			if err := replaceCompetitionScores(context.Background(), db, 1, "c", rows); err != nil {
				t.Fatalf("retry after %s: %v", phase, err)
			}
		})
	}
}
