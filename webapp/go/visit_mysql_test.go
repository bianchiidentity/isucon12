package isuports

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// Uses a disposable database, never the database named in the supplied DSN.
// The account needs CREATE/DROP DATABASE. Without a DSN, normal unit tests skip it.
func TestFirstVisitMySQL(t *testing.T) {
	dsn := os.Getenv("ISUCON_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set ISUCON_TEST_MYSQL_DSN for real MySQL integration tests")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = ""
	cfg.MultiStatements = true
	control, err := sqlx.Connect("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	schema, err := os.ReadFile("../sql/admin/10_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../sql/migrations/002_first_visit_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	initialize, err := os.ReadFile("../sql/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("migrate=%t", legacy), func(t *testing.T) {
			name := fmt.Sprintf("isucon_visit_test_%d", time.Now().UnixNano())
			if _, err := control.Exec("CREATE DATABASE `" + name + "`"); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := control.Exec("DROP DATABASE `" + name + "`"); err != nil {
					t.Error(err)
				}
			}()
			cfg.DBName = name
			db, err := sqlx.Connect("mysql", cfg.FormatDSN())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(8)
			ddl := strings.Replace(string(schema), "USE `isuports`;", "", 1)
			if legacy {
				ddl = strings.Replace(ddl, "  PRIMARY KEY (`tenant_id`, `competition_id`, `player_id`),\n", "", 1)
				ddl = strings.Replace(ddl, "  INDEX `visit_history_created_at` (`created_at`),\n", "", 1)
			}
			if _, err := db.Exec(ddl); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			const cutoff int64 = 1654041600
			// A baseline visit mixed with post-baseline visits must survive repeated init.
			for _, at := range []int64{cutoff + 30, cutoff - 1, cutoff + 60} {
				if legacy {
					if _, err := db.Exec("INSERT INTO visit_history VALUES ('baseline',1,'c',?,?)", at, at); err != nil {
						t.Fatal(err)
					}
				} else if err := recordFirstVisit(ctx, db, 1, "c", "baseline", at); err != nil {
					t.Fatal(err)
				}
			}
			if legacy {
				if _, err := db.Exec(string(migration)); err != nil {
					t.Fatal(err)
				}
			}
			assertFirst := func(tenant int64, competition, player string, want int64) {
				t.Helper()
				var times []int64
				if err := db.Select(&times, "SELECT created_at FROM visit_history WHERE tenant_id=? AND competition_id=? AND player_id=?", tenant, competition, player); err != nil {
					t.Fatal(err)
				}
				if len(times) != 1 || times[0] != want {
					t.Fatalf("%d/%s/%s: %v want %d", tenant, competition, player, times, want)
				}
			}
			assertFirst(1, "c", "baseline", cutoff-1)
			tenantDB := newRankingTestDB(t)
			addHistoryCompetition(t, tenantDB, 1, "c", "Competition", 0)
			if _, err := tenantDB.Exec("UPDATE competition SET finished_at=? WHERE id='c'", cutoff+100); err != nil {
				t.Fatal(err)
			}
			addRankingScore(t, tenantDB, 1, "c", "scored", 1, 10)
			t.Setenv("ISUCON_TENANT_DB_DIR", t.TempDir())
			old := adminDB
			adminDB = db
			defer func() { adminDB = old }()
			for run := 0; run < 2; run++ {
				var wg sync.WaitGroup
				errs := make(chan error, 64)
				for i := int64(63); i >= 0; i-- {
					wg.Add(1)
					go func(i int64) { defer wg.Done(); errs <- recordFirstVisit(ctx, db, 1, "parallel", "p", cutoff+10+i) }(i)
				}
				wg.Wait()
				close(errs)
				for err := range errs {
					if err != nil {
						t.Fatal(err)
					}
				}
				assertFirst(1, "parallel", "p", cutoff+10)
				for _, v := range []struct {
					tenant              int64
					competition, player string
					at                  int64
				}{
					{1, "c", "before", cutoff + 99}, {1, "c", "same", cutoff + 100}, {1, "c", "after", cutoff + 101},
					{1, "c", "scored", cutoff + 1}, {2, "c", "before", cutoff + 2}, {1, "other", "before", cutoff + 3},
					{1, "c", "baseline", cutoff + 1000}, {1, "c", "before", cutoff + 200},
				} {
					if err := recordFirstVisit(ctx, db, v.tenant, v.competition, v.player, v.at); err != nil {
						t.Fatal(err)
					}
				}
				assertFirst(1, "c", "before", cutoff+99)
				assertFirst(2, "c", "before", cutoff+2)
				assertFirst(1, "other", "before", cutoff+3)
				report, err := billingReportByCompetition(ctx, tenantDB, 1, "c")
				if err != nil || report.PlayerCount != 1 || report.VisitorCount != 3 || report.BillingYen != 130 {
					t.Fatalf("billing: %#v %v", report, err)
				}
				// Reuse the persisted report without reading SQLite scores.
				reads := tenantDB.selects
				cached, err := billingReportByCompetition(ctx, tenantDB, 1, "c")
				if err != nil || cached.BillingYen != 130 || tenantDB.selects != reads {
					t.Fatalf("cache not reused: %#v %v", cached, err)
				}
				// A same-second visit arriving after persistence must invalidate it.
				if err := recordFirstVisit(ctx, db, 1, "c", "delayed", cutoff+100); err != nil {
					t.Fatal(err)
				}
				updated, err := billingReportByCompetition(ctx, tenantDB, 1, "c")
				if err != nil || updated.BillingYen != 140 {
					t.Fatalf("delayed visit: %#v %v", updated, err)
				}
				// An earlier timestamp arriving later must also refresh the report.
				if err := recordFirstVisit(ctx, db, 1, "c", "after", cutoff+99); err != nil {
					t.Fatal(err)
				}
				updated, err = billingReportByCompetition(ctx, tenantDB, 1, "c")
				if err != nil || updated.BillingYen != 150 {
					t.Fatalf("earlier timestamp: %#v %v", updated, err)
				}
				// Repeated/later visits do not change the revision or rebuild scores.
				reads = tenantDB.selects
				if err := recordFirstVisit(ctx, db, 1, "c", "delayed", cutoff+200); err != nil {
					t.Fatal(err)
				}
				if _, err := billingReportByCompetition(ctx, tenantDB, 1, "c"); err != nil || tenantDB.selects != reads {
					t.Fatalf("no-op visit invalidated: %v", err)
				}
				// A visit arriving during calculation makes that saved revision stale.
				if _, err := db.Exec("DELETE FROM billing_reports WHERE tenant_id=1 AND competition_id='c'"); err != nil {
					t.Fatal(err)
				}
				racing := &billingRaceTestDB{rankingTestDB: tenantDB, hook: func() error { return recordFirstVisit(ctx, db, 1, "c", "during-calculation", cutoff+100) }}
				if _, err := billingReportByCompetition(ctx, racing, 1, "c"); err != nil {
					t.Fatal(err)
				}
				updated, err = billingReportByCompetition(ctx, tenantDB, 1, "c")
				if err != nil || updated.BillingYen != 160 {
					t.Fatalf("calculation race: %#v %v", updated, err)
				}
				// Failure to advance the revision must roll back the visit as well.
				if _, err := db.Exec("RENAME TABLE billing_versions TO billing_versions_test_backup"); err != nil {
					t.Fatal(err)
				}
				failure := recordFirstVisit(ctx, db, 1, "c", "rolled-back", cutoff+1)
				if _, err := db.Exec("RENAME TABLE billing_versions_test_backup TO billing_versions"); err != nil {
					t.Fatal(err)
				}
				var rolledBack int
				if err := db.Get(&rolledBack, "SELECT COUNT(*) FROM visit_history WHERE player_id='rolled-back'"); err != nil || failure == nil || rolledBack != 0 {
					t.Fatalf("atomic invalidation: count=%d err=%v failure=%v", rolledBack, err, failure)
				}
				if _, err := db.Exec(string(initialize)); err != nil {
					t.Fatal(err)
				}
				assertFirst(1, "c", "baseline", cutoff-1)
				var count int
				if err := db.Get(&count, "SELECT COUNT(*) FROM visit_history"); err != nil || count != 1 {
					t.Fatalf("reset count=%d err=%v", count, err)
				}
				report, err = billingReportByCompetition(ctx, tenantDB, 1, "c")
				if err != nil || report.PlayerCount != 1 || report.VisitorCount != 1 || report.BillingYen != 110 {
					t.Fatalf("reset billing: %#v %v", report, err)
				}
			}
		})
	}
}

type billingRaceTestDB struct {
	*rankingTestDB
	hook func() error
}

func (db *billingRaceTestDB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	if db.hook != nil {
		hook := db.hook
		db.hook = nil
		if err := hook(); err != nil {
			return err
		}
	}
	return db.rankingTestDB.SelectContext(ctx, dest, query, args...)
}
