package isuports

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func addHistoryCompetition(t *testing.T, db *rankingTestDB, tenant int64, id, title string, created int64) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO competition VALUES (?, ?, ?, NULL, ?, ?)", id, tenant, title, created, created); err != nil {
		t.Fatal(err)
	}
}

func TestPlayerScoresLatestRowOrderAndIsolation(t *testing.T) {
	db := newRankingTestDB(t)
	// Insert in a different order from both IDs and creation dates.
	addHistoryCompetition(t, db, 1, "a", "Later", 20)
	addHistoryCompetition(t, db, 1, "unscored", "No score", 15)
	addHistoryCompetition(t, db, 2, "foreign", "Other tenant", 5)
	addHistoryCompetition(t, db, 1, "z", "Earlier", 10)
	addRankingScore(t, db, 1, "a", "player", 1, 999)
	addRankingScore(t, db, 1, "a", "player", 2, 50)
	addRankingScore(t, db, 1, "z", "player", 1, -10)
	addRankingScore(t, db, 1, "z", "other-player", 2, 900)
	addRankingScore(t, db, 2, "z", "player", 3, 800)
	addRankingScore(t, db, 2, "foreign", "player", 1, 700)
	addRankingScore(t, db, 1, "missing-competition", "player", 1, 600)
	got, err := retrievePlayerScores(context.Background(), db, 1, "player")
	want := []PlayerScoreDetail{{CompetitionTitle: "Earlier", Score: -10}, {CompetitionTitle: "Later", Score: 50}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v %v; want %#v", got, err, want)
	}
	if db.selects != 1 || db.gets != 0 {
		t.Fatalf("SQL calls: selects=%d gets=%d", db.selects, db.gets)
	}
	// An overwrite removes previous rows, including players omitted from the CSV.
	if err := replaceCompetitionScores(context.Background(), db.DB, 1, "a", nil); err != nil {
		t.Fatal(err)
	}
	got, err = retrievePlayerScores(context.Background(), db, 1, "player")
	if err != nil || !reflect.DeepEqual(got, want[:1]) {
		t.Fatalf("after replacement: %#v %v", got, err)
	}
}

func TestPlayerScoresEmptyAndConstantQueryCount(t *testing.T) {
	db := newRankingTestDB(t)
	checkEmpty := func() {
		t.Helper()
		got, err := retrievePlayerScores(context.Background(), db, 1, "player")
		encoded, marshalErr := json.Marshal(got)
		if err != nil || marshalErr != nil || string(encoded) != "[]" {
			t.Fatalf("empty: %s %v %v", encoded, err, marshalErr)
		}
	}
	checkEmpty()
	for i := 0; i < 120; i++ {
		addHistoryCompetition(t, db, 1, fmt.Sprint(i), fmt.Sprint(i), int64(i))
	}
	checkEmpty()
	for i := 0; i < 120; i++ {
		addRankingScore(t, db, 1, fmt.Sprint(i), "player", 1, int64(i))
	}
	db.selects, db.gets = 0, 0
	got, err := retrievePlayerScores(context.Background(), db, 1, "player")
	if err != nil || len(got) != 120 {
		t.Fatalf("scores: %d %v", len(got), err)
	}
	for i, row := range got {
		if row.CompetitionTitle != fmt.Sprint(i) || row.Score != int64(i) {
			t.Fatalf("row %d: %#v", i, row)
		}
	}
	if db.selects != 1 || db.gets != 0 {
		t.Fatalf("SQL calls: selects=%d gets=%d", db.selects, db.gets)
	}
	// Record the actual plan against the production schema/index definitions.
	var plan []struct {
		ID     int    `db:"id"`
		Parent int    `db:"parent"`
		Unused int    `db:"notused"`
		Detail string `db:"detail"`
	}
	if err := db.DB.Select(&plan, "EXPLAIN QUERY PLAN "+playerScoresQuery, "player", 1); err != nil {
		t.Fatal(err)
	}
	for _, step := range plan {
		t.Log(step.Detail)
	}
	// A DB failure must not turn into a successful empty response.
	if _, err := db.Exec("DROP TABLE player_score"); err != nil {
		t.Fatal(err)
	}
	if _, err := retrievePlayerScores(context.Background(), db, 1, "player"); err == nil {
		t.Fatal("expected database error")
	}
}
