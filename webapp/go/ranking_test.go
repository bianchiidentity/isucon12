package isuports

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/jmoiron/sqlx"
)

type rankingTestDB struct {
	*sqlx.DB
	selects, gets int
}

func (db *rankingTestDB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	db.selects++
	return db.DB.SelectContext(ctx, dest, query, args...)
}
func (db *rankingTestDB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	db.gets++
	return db.DB.GetContext(ctx, dest, query, args...)
}
func newRankingTestDB(t *testing.T) *rankingTestDB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	schema, err := os.ReadFile("../sql/tenant/10_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	return &rankingTestDB{DB: db}
}
func addRankingPlayer(t *testing.T, db *rankingTestDB, id, name string, disqualified bool) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO player VALUES (?, 1, ?, ?, 0, 0)", id, name, disqualified); err != nil {
		t.Fatal(err)
	}
}
func addRankingScore(t *testing.T, db *rankingTestDB, tenant int64, competition, player string, row, score int64) {
	t.Helper()
	id := fmt.Sprintf("%d-%s-%d", tenant, competition, row)
	if _, err := db.Exec("INSERT INTO player_score VALUES (?, ?, ?, ?, ?, ?, 0, 0)", id, tenant, player, competition, score, row); err != nil {
		t.Fatal(err)
	}
}
func readRanking(t *testing.T, db *rankingTestDB, after int64) []CompetitionRank {
	t.Helper()
	db.selects, db.gets = 0, 0
	got, err := retrieveCompetitionRanks(context.Background(), db, 1, "c", after)
	if err != nil {
		t.Fatal(err)
	}
	if db.selects != 1 || db.gets != 0 {
		t.Fatalf("SQL calls: selects=%d gets=%d", db.selects, db.gets)
	}
	return got
}
func TestCompetitionRanksLatestTieAndDisplayName(t *testing.T) {
	db := newRankingTestDB(t)
	addRankingPlayer(t, db, "a", "名前変更後", false)
	addRankingPlayer(t, db, "b", "失格者もランキングに残る", true)
	addRankingPlayer(t, db, "empty", "", false)
	addRankingScore(t, db, 1, "c", "a", 1, 100)
	addRankingScore(t, db, 1, "c", "b", 2, 80)
	addRankingScore(t, db, 1, "c", "a", 3, 80)
	addRankingScore(t, db, 1, "c", "empty", 4, -1)
	// Neither another tenant nor another competition may leak into this ranking.
	addRankingScore(t, db, 2, "c", "a", 10, 999)
	addRankingScore(t, db, 1, "other", "a", 11, 999)
	want := []CompetitionRank{
		{Rank: 1, Score: 80, PlayerID: "b", PlayerDisplayName: "失格者もランキングに残る"},
		{Rank: 2, Score: 80, PlayerID: "a", PlayerDisplayName: "名前変更後"},
		{Rank: 3, Score: -1, PlayerID: "empty", PlayerDisplayName: ""},
	}
	if got := readRanking(t, db, 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if got := readRanking(t, db, 1); !reflect.DeepEqual(got, want[1:]) {
		t.Fatalf("rank_after: %#v", got)
	}
	if got := readRanking(t, db, -1); !reflect.DeepEqual(got, want) {
		t.Fatalf("negative rank_after: %#v", got)
	}
}
func TestCompetitionRanksPagingAndConstantQueryCount(t *testing.T) {
	db := newRankingTestDB(t)
	for i := int64(1); i <= 105; i++ {
		id := fmt.Sprint(i)
		addRankingPlayer(t, db, id, id, false)
		addRankingScore(t, db, 1, "c", id, i, 1000-i)
	}
	first := readRanking(t, db, 0)
	if len(first) != 100 || first[0].Rank != 1 || first[99].Rank != 100 {
		t.Fatalf("first page: %#v", first)
	}
	last := readRanking(t, db, 100)
	if len(last) != 5 || last[0].Rank != 101 || last[4].Rank != 105 {
		t.Fatalf("last page: %#v", last)
	}
	if got := readRanking(t, db, 105); len(got) != 0 || got == nil {
		t.Fatalf("past end: %#v", got)
	}
	// Missing references outside the returned page must still produce an error.
	addRankingScore(t, db, 1, "c", "missing", 106, -999)
	_, err := retrieveCompetitionRanks(context.Background(), db, 1, "c", 0)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing player error: %v", err)
	}
}
func TestCompetitionRanksEmpty(t *testing.T) {
	db := newRankingTestDB(t)
	got := readRanking(t, db, 0)
	encoded, err := json.Marshal(got)
	if err != nil || string(encoded) != "[]" {
		t.Fatalf("empty ranking: %s %v", encoded, err)
	}
}
