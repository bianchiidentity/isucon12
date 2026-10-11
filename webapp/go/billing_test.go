package isuports

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBillingSkipsOngoingAndCalculatesAfterFinish(t *testing.T) {
	ctx := context.Background()
	db := newRankingTestDB(t)
	addHistoryCompetition(t, db, 1, "c", "Competition", 0)
	// A nil admin DB and nonexistent lock directory must not affect ongoing billing.
	old := adminDB
	adminDB = nil
	defer func() { adminDB = old }()
	t.Setenv("ISUCON_TENANT_DB_DIR", filepath.Join(t.TempDir(), "missing"))
	report, err := billingReportByCompetition(ctx, db, 1, "c")
	want := &BillingReport{CompetitionID: "c", CompetitionTitle: "Competition"}
	if err != nil || !reflect.DeepEqual(report, want) {
		t.Fatalf("ongoing: %#v %v", report, err)
	}
	if db.gets != 1 || db.selects != 0 {
		t.Fatalf("ongoing queries: gets=%d selects=%d", db.gets, db.selects)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"player_count", "visitor_count", "billing_player_yen", "billing_visitor_yen", "billing_yen"} {
		if value, ok := fields[key]; !ok || value != float64(0) {
			t.Fatalf("missing/invalid zero field %s: %s", key, encoded)
		}
	}
	if _, err := billingReportByCompetition(ctx, db, 1, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing competition: %v", err)
	}
	// The billing query is exercised on SQLite as a test double for MySQL.
	adminDB = db.DB
	if _, err := db.Exec(`CREATE TABLE billing_versions (tenant_id INTEGER, competition_id TEXT, version INTEGER, PRIMARY KEY(tenant_id,competition_id));
 CREATE TABLE billing_reports (tenant_id INTEGER, competition_id TEXT, finished_at INTEGER, version INTEGER, player_count INTEGER, visitor_count INTEGER, PRIMARY KEY(tenant_id,competition_id));
 CREATE TABLE visit_history (tenant_id INTEGER, competition_id TEXT, player_id TEXT, created_at INTEGER, PRIMARY KEY (tenant_id, competition_id, player_id));
 INSERT INTO visit_history VALUES
 (1,'c','scored',1), (1,'c','visitor',99),
 (1,'c','same-second',100), (1,'c','too-late',101),
 (2,'c','other-tenant',1), (1,'other','other-competition',1);
 UPDATE competition SET finished_at=100 WHERE id='c'`); err != nil {
		t.Fatal(err)
	}
	addRankingScore(t, db, 1, "c", "scored", 1, 10)
	addRankingScore(t, db, 1, "c", "scored", 2, 20)
	t.Setenv("ISUCON_TENANT_DB_DIR", t.TempDir())
	report, err = billingReportByCompetition(ctx, db, 1, "c")
	want = &BillingReport{CompetitionID: "c", CompetitionTitle: "Competition", PlayerCount: 1, VisitorCount: 2, BillingPlayerYen: 100, BillingVisitorYen: 20, BillingYen: 120}
	if err != nil || !reflect.DeepEqual(report, want) {
		t.Fatalf("finished: %#v %v", report, err)
	}
	// A non-NULL zero timestamp is finished as well: use Valid, not Int64 > 0.
	if _, err := db.Exec("UPDATE competition SET finished_at=0 WHERE id='c'"); err != nil {
		t.Fatal(err)
	}
	report, err = billingReportByCompetition(ctx, db, 1, "c")
	want.VisitorCount = 0
	want.BillingVisitorYen = 0
	want.BillingYen = 100
	if err != nil || !reflect.DeepEqual(report, want) {
		t.Fatalf("finished at zero: %#v %v", report, err)
	}
}
