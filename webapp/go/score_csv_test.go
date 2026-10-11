package isuports

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestReadCompetitionScoresLastRow(t *testing.T) {
	db := newRankingTestDB(t)
	addRankingPlayer(t, db, "a", "A", false)
	addRankingPlayer(t, db, "b", "B", true)
	addRankingScore(t, db, 1, "c", "gone", 1, 999)
	ctx := context.Background()
	rows, count, err := readCompetitionScores(ctx, db, 1, "c", csv.NewReader(strings.NewReader("a,100\nb,80\na,80\n")))
	if err != nil || count != 3 || len(rows) != 2 || db.gets != 2 {
		t.Fatalf("rows=%#v count=%d gets=%d err=%v", rows, count, db.gets, err)
	}
	if rows[0].PlayerID != "a" || rows[0].Score != 80 || rows[0].RowNum != 3 || rows[1].RowNum != 2 || rows[0].ID == "" || rows[0].ID == rows[1].ID {
		t.Fatalf("last row/IDs: %#v", rows)
	}
	if err := replaceCompetitionScores(ctx, db.DB, 1, "c", rows); err != nil {
		t.Fatal(err)
	}
	ranks, err := retrieveCompetitionRanks(ctx, db, 1, "c", 0)
	if err != nil || len(ranks) != 2 || ranks[0].PlayerID != "b" || ranks[1].PlayerID != "a" || len(scoreRows(t, db.DB)) != 2 {
		t.Fatalf("replacement/tie order: %#v %v", ranks, err)
	}
	rows, count, err = readCompetitionScores(ctx, db, 1, "c", csv.NewReader(strings.NewReader("")))
	if err != nil || count != 0 || len(rows) != 0 {
		t.Fatalf("empty: %#v %d %v", rows, count, err)
	}
	if err := replaceCompetitionScores(ctx, db.DB, 1, "c", rows); err != nil {
		t.Fatal(err)
	}
	if got := scoreRows(t, db.DB); len(got) != 0 {
		t.Fatalf("empty replacement: %#v", got)
	}
}

func TestReadCompetitionScoresRejectsEveryInvalidRow(t *testing.T) {
	for _, tc := range []struct {
		name, csv string
		status    int
	}{
		{"invalid discarded row", "a,bad\na,10\n", 400},
		{"invalid repeated row", "a,10\na,bad\n", 400},
		{"overflow", "a,9223372036854775808\na,10\n", 400},
		{"unknown player", "missing,10\na,10\n", 400},
		{"unknown repeated player", "a,10\nmissing,10\nmissing,20\n", 400},
		{"missing column", "a\na,10\n", 0},
		{"extra column", "a,10,extra\n", 0},
		{"broken CSV", "a,10\n\"a,20\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newRankingTestDB(t)
			addRankingPlayer(t, db, "a", "A", false)
			addRankingScore(t, db, 1, "c", "a", 1, 999)
			before := scoreRows(t, db.DB)
			rows, _, err := readCompetitionScores(context.Background(), db, 1, "c", csv.NewReader(strings.NewReader(tc.csv)))
			if err == nil || rows != nil {
				t.Fatalf("invalid input accepted: %#v %v", rows, err)
			}
			if tc.status != 0 {
				var httpErr *echo.HTTPError
				if !errors.As(err, &httpErr) || httpErr.Code != http.StatusBadRequest {
					t.Fatalf("error status: %v", err)
				}
			}
			if !reflect.DeepEqual(before, scoreRows(t, db.DB)) {
				t.Fatal("invalid input changed scores")
			}
		})
	}
}

func TestReadCompetitionScoresLargeCSV(t *testing.T) {
	db := newRankingTestDB(t)
	const players, repeats = 100, 100
	for i := 0; i < players; i++ {
		addRankingPlayer(t, db, fmt.Sprint(i), "Player", false)
	}
	var input strings.Builder
	for n := 0; n < repeats; n++ {
		for i := 0; i < players; i++ {
			fmt.Fprintf(&input, "%d,%d\n", i, -n)
		}
	}
	rows, count, err := readCompetitionScores(context.Background(), db, 1, "c", csv.NewReader(strings.NewReader(input.String())))
	if err != nil || count != players*repeats || len(rows) != players || db.gets != players {
		t.Fatalf("count=%d saved=%d gets=%d err=%v", count, len(rows), db.gets, err)
	}
	for i, row := range rows {
		if row.Score != -(repeats-1) || row.RowNum != int64((repeats-1)*players+i+1) {
			t.Fatalf("last row: %#v", row)
		}
	}
}

func TestCompactInitialScores(t *testing.T) {
	db := newRankingTestDB(t)
	addRankingPlayer(t, db, "a", "A", false)
	addRankingPlayer(t, db, "b", "B", false)
	addRankingScore(t, db, 1, "c", "a", 1, 999)
	addRankingScore(t, db, 1, "c", "b", 2, 80)
	addRankingScore(t, db, 1, "c", "a", 3, 80)
	addRankingScore(t, db, 1, "other", "a", 1, 222)
	addRankingScore(t, db, 2, "c", "a", 1, 333)
	before, err := retrieveCompetitionRanks(context.Background(), db, 1, "c", 0)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../sql/tenant/30_compact_scores.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatal(err)
		}
		after, err := retrieveCompetitionRanks(context.Background(), db, 1, "c", 0)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("rank changed: %#v %v", after, err)
		}
		if got := scoreRows(t, db.DB); len(got) != 4 {
			t.Fatalf("partition/idempotence: %#v", got)
		}
	}
}
