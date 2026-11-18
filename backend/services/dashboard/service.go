package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const cacheTTL = 180 * time.Second

// Service computes real-DB ERP KPIs, FY (Apr-Mar) analytics, 180s metrics caching,
// and daily KPI snapshot persistence without fabricated figures.
type Service struct {
	db    *sql.DB
	sec   *security.Service
	audit *audit.Service
}

func NewService(db *sql.DB, sec *security.Service, aud *audit.Service) *Service {
	return &Service{db: db, sec: sec, audit: aud}
}

// ResolveFinancialYearBounds converts an Indian FY string ("2026-27") to Apr 1..Mar 31 bounds.
func ResolveFinancialYearBounds(fy, startOverride, endOverride string) (string, string, string) {
	fyClean := strings.TrimSpace(fy)
	s := strings.TrimSpace(startOverride)
	e := strings.TrimSpace(endOverride)
	if s != "" || e != "" {
		return fyClean, s, e
	}
	if fyClean == "" || strings.EqualFold(fyClean, "all") {
		return "all", "", ""
	}
	parts := strings.Split(fyClean, "-")
	if len(parts) >= 1 {
		startYr, err := strconv.Atoi(parts[0])
		if err == nil && startYr >= 2000 && startYr <= 2100 {
			return fyClean, fmt.Sprintf("%04d-04-01", startYr), fmt.Sprintf("%04d-03-31", startYr+1)
		}
	}
	return fyClean, "", ""
}

// GetOverview returns cached (within 180s) or freshly computed real-database ERP metrics.
func (s *Service) GetOverview(filter models.DashboardFilter) (models.DashboardOverview, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.DashboardOverview{}, err
	}

	fy, startDate, endDate := ResolveFinancialYearBounds(filter.FinancialYear, filter.StartDate, filter.EndDate)
	cacheKey := fmt.Sprintf("fy=%s|s=%s|e=%s", fy, startDate, endDate)

	if !filter.ForceRefresh {
		if cached, ok := s.loadCached(cacheKey); ok {
			return cached, nil
		}
	}

	overview, err := s.computeLive(fy, startDate, endDate)
	if err != nil {
		return models.DashboardOverview{}, err
	}
	s.saveCached(cacheKey, overview)
	return overview, nil
}

// InvalidateCache clears all entries in dashboard_metrics_cache.
func (s *Service) InvalidateCache() error {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM dashboard_metrics_cache`)
	return err
}

// RebuildDailySnapshot persists today's KPI, monthly, exception, and daily series snapshots.
func (s *Service) RebuildDailySnapshot() (string, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return "", err
	}

	overview, err := s.computeLive("all", "", "")
	if err != nil {
		return "", err
	}

	snapDate := time.Now().UTC().Format("2006-01-02")
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO dashboard_kpi_snapshot (
				snapshot_date, total_shipments, pending_shipments, delivered_shipments,
				overdue_eta_count, shipments_missing_boe, shipments_missing_expense,
				total_invoice_value, total_expense_value, total_duty_savings,
				reconciled_boes, duty_total, total_suppliers
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)
			ON CONFLICT(snapshot_date) DO UPDATE SET
				total_shipments = excluded.total_shipments,
				pending_shipments = excluded.pending_shipments,
				delivered_shipments = excluded.delivered_shipments,
				overdue_eta_count = excluded.overdue_eta_count,
				shipments_missing_boe = excluded.shipments_missing_boe,
				shipments_missing_expense = excluded.shipments_missing_expense,
				total_invoice_value = excluded.total_invoice_value,
				total_expense_value = excluded.total_expense_value,
				reconciled_boes = excluded.reconciled_boes,
				duty_total = excluded.duty_total,
				total_suppliers = excluded.total_suppliers`,
			snapDate, overview.KPIs.TotalShipments, overview.KPIs.PendingShipments, overview.KPIs.CompletedShipments,
			overview.KPIs.OverdueETACount, overview.KPIs.ShipmentsMissingBOE, overview.KPIs.ShipmentsMissingExpense,
			overview.KPIs.TotalImportValueINR, overview.KPIs.TotalBasicExpensesINR,
			overview.KPIs.ReconciledBOEs, overview.KPIs.TotalCustomsDutyINR, overview.KPIs.TotalSuppliers,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO dashboard_exception_snapshot (
				snapshot_date, missing_boe_count, missing_expense_count, overdue_eta_count,
				open_exception_count, sla_breach_count
			) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(snapshot_date) DO UPDATE SET
				missing_boe_count = excluded.missing_boe_count,
				missing_expense_count = excluded.missing_expense_count,
				overdue_eta_count = excluded.overdue_eta_count,
				open_exception_count = excluded.open_exception_count,
				sla_breach_count = excluded.sla_breach_count`,
			snapDate, overview.KPIs.ShipmentsMissingBOE, overview.KPIs.ShipmentsMissingExpense,
			overview.KPIs.OverdueETACount, overview.KPIs.OpenExceptionCount, overview.KPIs.SLABreachCount,
		); err != nil {
			return err
		}

		for _, m := range overview.MonthlyTrends {
			if _, err := tx.Exec(`
				INSERT INTO dashboard_monthly_snapshot (snapshot_date, period, shipments, value, duty_savings)
				VALUES (?, ?, ?, ?, 0)
				ON CONFLICT(snapshot_date, period) DO UPDATE SET
					shipments = excluded.shipments,
					value = excluded.value`,
				snapDate, m.Period, m.Shipments, m.ImportValueINR,
			); err != nil {
				return err
			}
		}

		_, err := tx.Exec(`DELETE FROM dashboard_metrics_cache`)
		return err
	})
	if err != nil {
		return "", err
	}
	_ = s.audit.RecordTableAudit("dashboard_kpi_snapshot", snapDate, "REBUILD", sess.UserID, nil, overview.KPIs, nil)
	return snapDate, nil
}

func (s *Service) computeLive(fy, startDate, endDate string) (models.DashboardOverview, error) {
	today := time.Now().UTC().Format("2006-01-02")
	var kpis models.DashboardKPIs

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM suppliers WHERE deleted_at IS NULL`).Scan(&kpis.TotalSuppliers); err != nil {
		return models.DashboardOverview{}, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM suppliers WHERE deleted_at IS NULL AND COALESCE(is_active,1) = 1`).Scan(&kpis.ActiveSuppliers); err != nil {
		return models.DashboardOverview{}, err
	}

	dateFilter, dateArgs := buildDateFilter("s.invoice_date", startDate, endDate)

	// Shipments + counts
	shipRows, err := s.db.Query(`
		SELECT s.id, s.supplier_id, COALESCE(sup.supplier_name,''), COALESCE(s.invoice_date,''),
		       LOWER(COALESCE(s.status,'docs-rcvd')), COALESCE(s.eta,''),
		       EXISTS(SELECT 1 FROM boe_calculations bc WHERE bc.shipment_id = s.id AND bc.deleted_at IS NULL),
		       EXISTS(SELECT 1 FROM expense_invoices ei WHERE ei.shipment_id = s.id AND ei.deleted_at IS NULL)
		FROM shipments s
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		WHERE s.deleted_at IS NULL`+dateFilter, dateArgs...)
	if err != nil {
		return models.DashboardOverview{}, err
	}
	defer shipRows.Close()

	statusCounts := make(map[string]int)
	monthMap := make(map[string]*models.DashboardMonthlyTrend)
	supMap := make(map[string]*models.DashboardSupplierSummary)
	shipSupplier := make(map[string]string)
	shipSupplierName := make(map[string]string)
	shipMonth := make(map[string]string)

	for shipRows.Next() {
		var sid, supID, supName, invDate, st, eta string
		var hasBOE, hasExp bool
		if err := shipRows.Scan(&sid, &supID, &supName, &invDate, &st, &eta, &hasBOE, &hasExp); err != nil {
			return models.DashboardOverview{}, err
		}
		kpis.TotalShipments++
		if isTerminalStatus(st) {
			kpis.CompletedShipments++
		} else {
			kpis.PendingShipments++
			if eta != "" && eta < today {
				kpis.OverdueETACount++
			}
		}
		if !hasBOE && st != "cancelled" {
			kpis.ShipmentsMissingBOE++
		}
		if !hasExp && st != "cancelled" {
			kpis.ShipmentsMissingExpense++
		}
		statusCounts[st]++

		month := ""
		if len(invDate) >= 7 {
			month = invDate[:7]
			mt, ok := monthMap[month]
			if !ok {
				mt = &models.DashboardMonthlyTrend{Period: month}
				monthMap[month] = mt
			}
			mt.Shipments++
		}
		shipMonth[sid] = month
		shipSupplier[sid] = supID
		shipSupplierName[sid] = supName

		if supID != "" {
			ss, ok := supMap[supID]
			if !ok {
				ss = &models.DashboardSupplierSummary{SupplierID: supID, SupplierName: supName}
				supMap[supID] = ss
			}
			ss.ShipmentCount++
		}
	}
	if err := shipRows.Err(); err != nil {
		return models.DashboardOverview{}, err
	}
	_ = shipRows.Close()

	// Invoices
	invFilter, invArgs := buildDateFilter("s.invoice_date", startDate, endDate)
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM invoices i
		JOIN shipments s ON s.id = i.shipment_id AND s.deleted_at IS NULL
		WHERE i.deleted_at IS NULL`+invFilter, invArgs...).Scan(&kpis.TotalInvoices); err != nil {
		return models.DashboardOverview{}, err
	}

	// BOE Calculations (assessable + customs duty INR)
	boeFilter, boeArgs := buildDateFilter("s.invoice_date", startDate, endDate)
	boeRows, err := s.db.Query(`
		SELECT bc.shipment_id, bc.status,
		       COALESCE(CAST(json_extract(bc.calculation_result_json, '$.totalAssessableValue') AS REAL), 0),
		       COALESCE(CAST(json_extract(bc.calculation_result_json, '$.customsDutyTotal') AS REAL), 0)
		FROM boe_calculations bc
		JOIN shipments s ON s.id = bc.shipment_id AND s.deleted_at IS NULL
		WHERE bc.deleted_at IS NULL`+boeFilter, boeArgs...)
	if err != nil {
		return models.DashboardOverview{}, err
	}
	defer boeRows.Close()

	for boeRows.Next() {
		var sid, st string
		var av, duty float64
		if err := boeRows.Scan(&sid, &st, &av, &duty); err != nil {
			return models.DashboardOverview{}, err
		}
		if st == "Reconciled" || st == "Closed" {
			kpis.ReconciledBOEs++
		} else {
			kpis.PendingBOEs++
		}
		kpis.TotalImportValueINR += av
		kpis.TotalCustomsDutyINR += duty
		if m := shipMonth[sid]; m != "" && monthMap[m] != nil {
			monthMap[m].ImportValueINR += av
			monthMap[m].CustomsDutyINR += duty
		}
		if supID := shipSupplier[sid]; supID != "" && supMap[supID] != nil {
			supMap[supID].TotalAssessableINR += av
			supMap[supID].TotalDutyINR += duty
		}
	}
	if err := boeRows.Err(); err != nil {
		return models.DashboardOverview{}, err
	}
	_ = boeRows.Close()

	// Expenses (basic & gross INR via exact paise columns)
	expFilter, expArgs := buildDateFilter("e.invoice_date", startDate, endDate)
	expRows, err := s.db.Query(`
		SELECT COALESCE(e.invoice_date, ''),
		       COALESCE(NULLIF(e.amount_paise,0), CAST(ROUND(COALESCE(e.amount,0)*100) AS INTEGER), 0),
		       COALESCE(NULLIF(e.total_amount_paise,0), CAST(ROUND(COALESCE(e.total_amount, e.amount, 0)*100) AS INTEGER), 0)
		FROM expenses e
		JOIN expense_invoices ei ON ei.id = e.expense_invoice_id AND ei.deleted_at IS NULL
		WHERE e.deleted_at IS NULL`+expFilter, expArgs...)
	if err != nil {
		return models.DashboardOverview{}, err
	}
	defer expRows.Close()

	var totalBasicPaise, totalGrossPaise int64
	for expRows.Next() {
		var invDate string
		var basicP, grossP int64
		if err := expRows.Scan(&invDate, &basicP, &grossP); err != nil {
			return models.DashboardOverview{}, err
		}
		totalBasicPaise += basicP
		totalGrossPaise += grossP
		if len(invDate) >= 7 {
			m := invDate[:7]
			mt, ok := monthMap[m]
			if !ok {
				mt = &models.DashboardMonthlyTrend{Period: m}
				monthMap[m] = mt
			}
			mt.BasicExpensesINR += float64(basicP) / 100.0
		}
	}
	if err := expRows.Err(); err != nil {
		return models.DashboardOverview{}, err
	}
	_ = expRows.Close()
	kpis.TotalBasicExpensesINR = float64(totalBasicPaise) / 100.0
	kpis.TotalGrossExpensesINR = float64(totalGrossPaise) / 100.0

	// Exceptions & Incidents
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM exception_cases WHERE UPPER(status) IN ('OPEN','IN_PROGRESS')`).Scan(&kpis.OpenExceptionCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM exception_cases WHERE UPPER(status) IN ('OPEN','IN_PROGRESS') AND UPPER(sla_status) = 'BREACHED'`).Scan(&kpis.SLABreachCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM workflow_incidents WHERE deleted_at IS NULL AND UPPER(status) IN ('OPEN','INVESTIGATING')`).Scan(&kpis.OpenIncidentCount)

	// Health score formula matching legacy clamp [0, 100]
	rawHealth := 100.0 -
		math.Min(float64(kpis.OpenExceptionCount)*0.4, 25.0) -
		math.Min(float64(kpis.SLABreachCount)*1.5, 20.0) -
		math.Min(float64(kpis.OpenIncidentCount)*5.0, 25.0)
	if rawHealth < 0 {
		rawHealth = 0
	}
	kpis.WorkflowHealthScore = math.Round(rawHealth*10) / 10

	trends := make([]models.DashboardMonthlyTrend, 0, len(monthMap))
	for _, m := range monthMap {
		trends = append(trends, *m)
	}
	sort.Slice(trends, func(i, j int) bool { return trends[i].Period < trends[j].Period })

	suppliers := make([]models.DashboardSupplierSummary, 0, len(supMap))
	for _, sp := range supMap {
		suppliers = append(suppliers, *sp)
	}
	sort.Slice(suppliers, func(i, j int) bool { return suppliers[i].TotalAssessableINR > suppliers[j].TotalAssessableINR })
	if len(suppliers) > 10 {
		suppliers = suppliers[:10]
	}

	statuses := make([]models.DashboardStatusCount, 0, len(statusCounts))
	for st, c := range statusCounts {
		statuses = append(statuses, models.DashboardStatusCount{Status: st, Count: c})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Count > statuses[j].Count })

	activity, err := s.loadRecentActivity()
	if err != nil {
		return models.DashboardOverview{}, err
	}

	alerts := buildAlerts(kpis)
	fys := s.listAvailableFYs()

	var lastSnap string
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(snapshot_date),'') FROM dashboard_kpi_snapshot`).Scan(&lastSnap)

	return models.DashboardOverview{
		FinancialYear:     fy,
		StartDate:         startDate,
		EndDate:           endDate,
		AvailableFYs:      fys,
		KPIs:              kpis,
		MonthlyTrends:     trends,
		SupplierBreakdown: suppliers,
		StatusBreakdown:   statuses,
		RecentActivity:    activity,
		Alerts:            alerts,
		CacheHit:          false,
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		LastSnapshotDate:  lastSnap,
	}, nil
}

func (s *Service) loadRecentActivity() ([]models.DashboardActivityItem, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(table_name,''), COALESCE(row_id,''), COALESCE(action,''),
		       COALESCE(user_id,''), COALESCE(created_at,'')
		FROM audit_logs
		ORDER BY id DESC
		LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.DashboardActivityItem
	for rows.Next() {
		var a models.DashboardActivityItem
		if err := rows.Scan(&a.ID, &a.TableName, &a.RowID, &a.Action, &a.UserID, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) listAvailableFYs() []string {
	fySet := map[string]struct{}{}
	rows, err := s.db.Query(`SELECT DISTINCT invoice_date FROM shipments WHERE deleted_at IS NULL AND invoice_date != ''`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			if rows.Scan(&d) == nil && len(d) >= 7 {
				yr, err1 := strconv.Atoi(d[:4])
				mo, err2 := strconv.Atoi(d[5:7])
				if err1 == nil && err2 == nil {
					if mo < 4 {
						yr--
					}
					fySet[fmt.Sprintf("%04d-%02d", yr, (yr+1)%100)] = struct{}{}
				}
			}
		}
	}
	now := time.Now().UTC()
	curYr := now.Year()
	if now.Month() < 4 {
		curYr--
	}
	fySet[fmt.Sprintf("%04d-%02d", curYr, (curYr+1)%100)] = struct{}{}

	out := make([]string, 0, len(fySet))
	for k := range fySet {
		out = append(out, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

func (s *Service) loadCached(key string) (models.DashboardOverview, bool) {
	var payload, expiresAt string
	err := s.db.QueryRow(`SELECT metrics_payload, expires_at FROM dashboard_metrics_cache WHERE cache_key = ?`, key).Scan(&payload, &expiresAt)
	if err != nil {
		return models.DashboardOverview{}, false
	}
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().UTC().After(exp) {
		return models.DashboardOverview{}, false
	}
	var ov models.DashboardOverview
	if json.Unmarshal([]byte(payload), &ov) != nil {
		return models.DashboardOverview{}, false
	}
	ov.CacheHit = true
	return ov, true
}

func (s *Service) saveCached(key string, ov models.DashboardOverview) {
	blob, err := json.Marshal(ov)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	exp := now.Add(cacheTTL)
	_, _ = s.db.Exec(`
		INSERT INTO dashboard_metrics_cache (cache_key, metrics_payload, snapshot_at, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
			metrics_payload = excluded.metrics_payload,
			snapshot_at = excluded.snapshot_at,
			expires_at = excluded.expires_at`,
		key, string(blob), now.Format(time.RFC3339), exp.Format(time.RFC3339),
	)
}

func buildDateFilter(col, start, end string) (string, []any) {
	var clause strings.Builder
	var args []any
	if start != "" {
		clause.WriteString(" AND " + col + " >= ?")
		args = append(args, start)
	}
	if end != "" {
		clause.WriteString(" AND " + col + " <= ?")
		args = append(args, end)
	}
	return clause.String(), args
}

func isTerminalStatus(st string) bool {
	switch st {
	case "delivered", "completed", "finalized", "closed", "cancelled":
		return true
	default:
		return false
	}
}

func buildAlerts(kpis models.DashboardKPIs) []models.DashboardAlertItem {
	alerts := make([]models.DashboardAlertItem, 0)
	if kpis.OverdueETACount > 0 {
		alerts = append(alerts, models.DashboardAlertItem{
			Kind: "OVERDUE_ETA", Severity: "critical",
			Title: "Shipments past ETA", Count: kpis.OverdueETACount, NavigationTarget: "/shipments",
		})
	}
	if kpis.ShipmentsMissingBOE > 0 {
		alerts = append(alerts, models.DashboardAlertItem{
			Kind: "MISSING_BOE", Severity: "warning",
			Title: "Shipments awaiting BOE calculation", Count: kpis.ShipmentsMissingBOE, NavigationTarget: "/boe-entry",
		})
	}
	if kpis.ShipmentsMissingExpense > 0 {
		alerts = append(alerts, models.DashboardAlertItem{
			Kind: "MISSING_EXPENSE", Severity: "warning",
			Title: "Shipments missing import expenses", Count: kpis.ShipmentsMissingExpense, NavigationTarget: "/expenses",
		})
	}
	if kpis.SLABreachCount > 0 {
		alerts = append(alerts, models.DashboardAlertItem{
			Kind: "SLA_BREACH", Severity: "critical",
			Title: "Workflow exceptions past SLA deadline", Count: kpis.SLABreachCount, NavigationTarget: "/admin/operations-center",
		})
	}
	return alerts
}
