package isuports

import "context"

type billingCounts struct {
	PlayerCount  int64 `db:"player_count"`
	VisitorCount int64 `db:"visitor_count"`
}

func makeBillingReport(comp *CompetitionRow, players, visitors int64) *BillingReport {
	return &BillingReport{CompetitionID: comp.ID, CompetitionTitle: comp.Title,
		PlayerCount: players, VisitorCount: visitors, BillingPlayerYen: 100 * players,
		BillingVisitorYen: 10 * visitors, BillingYen: 100*players + 10*visitors}
}

func loadBillingCounts(ctx context.Context, tenantID int64, comp *CompetitionRow) (billingCounts, error) {
	var counts billingCounts
	err := adminDB.GetContext(ctx, &counts, `SELECT r.player_count, r.visitor_count FROM billing_reports r
 LEFT JOIN billing_versions v ON v.tenant_id=r.tenant_id AND v.competition_id=r.competition_id
 WHERE r.tenant_id=? AND r.competition_id=? AND r.finished_at=? AND r.version=COALESCE(v.version,0)`, tenantID, comp.ID, comp.FinishedAt.Int64)
	return counts, err
}
