package models

// DashboardFilter scopes KPI analytics by Indian Financial Year (Apr-Mar) or date range.
type DashboardFilter struct {
	FinancialYear string `json:"financialYear"` // e.g. "2026-27" or "all"
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	ForceRefresh  bool   `json:"forceRefresh"`
}

// DashboardKPIs summarizes real database counts and INR financial sums across ERP modules.
type DashboardKPIs struct {
	TotalSuppliers          int     `json:"totalSuppliers"`
	ActiveSuppliers         int     `json:"activeSuppliers"`
	TotalShipments          int     `json:"totalShipments"`
	PendingShipments        int     `json:"pendingShipments"`
	CompletedShipments      int     `json:"completedShipments"`
	OverdueETACount         int     `json:"overdueEtaCount"`
	ShipmentsMissingBOE     int     `json:"shipmentsMissingBoe"`
	ShipmentsMissingExpense int     `json:"shipmentsMissingExpense"`
	TotalInvoices           int     `json:"totalInvoices"`
	PendingBOEs             int     `json:"pendingBoes"`
	ReconciledBOEs          int     `json:"reconciledBoes"`
	TotalImportValueINR     float64 `json:"totalImportValueInr"`
	TotalCustomsDutyINR     float64 `json:"totalCustomsDutyInr"`
	TotalBasicExpensesINR   float64 `json:"totalBasicExpensesInr"`
	TotalGrossExpensesINR   float64 `json:"totalGrossExpensesInr"`
	OpenExceptionCount      int     `json:"openExceptionCount"`
	SLABreachCount          int     `json:"slaBreachCount"`
	OpenIncidentCount       int     `json:"openIncidentCount"`
	WorkflowHealthScore     float64 `json:"workflowHealthScore"`
}

// DashboardMonthlyTrend captures month-by-month shipment volume, assessable import value, customs duty & basic spend.
type DashboardMonthlyTrend struct {
	Period           string  `json:"period"` // YYYY-MM
	Shipments        int     `json:"shipments"`
	ImportValueINR   float64 `json:"importValueInr"`
	CustomsDutyINR   float64 `json:"customsDutyInr"`
	BasicExpensesINR float64 `json:"basicExpensesInr"`
}

// DashboardSupplierSummary breaks down import activity by supplier.
type DashboardSupplierSummary struct {
	SupplierID         string  `json:"supplierId"`
	SupplierName       string  `json:"supplierName"`
	ShipmentCount      int     `json:"shipmentCount"`
	TotalAssessableINR float64 `json:"totalAssessableInr"`
	TotalDutyINR       float64 `json:"totalDutyInr"`
}

// DashboardStatusCount breaks down shipment counts by lifecycle status.
type DashboardStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// DashboardActivityItem displays recent audit trail mutations.
type DashboardActivityItem struct {
	ID        int64  `json:"id"`
	TableName string `json:"tableName"`
	RowID     string `json:"rowId"`
	Action    string `json:"action"`
	UserID    string `json:"userId"`
	CreatedAt string `json:"createdAt"`
}

// DashboardAlertItem highlights actionable operational exceptions.
type DashboardAlertItem struct {
	Kind             string `json:"kind"`
	Severity         string `json:"severity"` // info, warning, critical
	Title            string `json:"title"`
	Count            int    `json:"count"`
	NavigationTarget string `json:"navigationTarget"`
}

// DashboardOverview bundles all live KPIs, charts, alerts, and cache status.
type DashboardOverview struct {
	FinancialYear      string                     `json:"financialYear"`
	StartDate          string                     `json:"startDate"`
	EndDate            string                     `json:"endDate"`
	AvailableFYs       []string                   `json:"availableFys"`
	KPIs               DashboardKPIs              `json:"kpis"`
	MonthlyTrends      []DashboardMonthlyTrend    `json:"monthlyTrends"`
	SupplierBreakdown  []DashboardSupplierSummary `json:"supplierBreakdown"`
	StatusBreakdown    []DashboardStatusCount     `json:"statusBreakdown"`
	RecentActivity     []DashboardActivityItem    `json:"recentActivity"`
	Alerts             []DashboardAlertItem       `json:"alerts"`
	CacheHit           bool                       `json:"cacheHit"`
	GeneratedAt        string                     `json:"generatedAt"`
	LastSnapshotDate   string                     `json:"lastSnapshotDate"`
}

// ModuleDisplaySettings customizes per-module field case, precision, visibility, and pagination.
type ModuleDisplaySettings struct {
	VisibleFields    []string          `json:"visibleFields"`
	ItemsPerPage     int               `json:"itemsPerPage"`
	DecimalPrecision int               `json:"decimalPrecision"`
	DateFormat       string            `json:"dateFormat"`
	FieldCases       map[string]string `json:"fieldCases"`
}

// AppSettingsProfile mirrors all persistent ERP formatting, precision, and appearance settings.
type AppSettingsProfile struct {
	SchemaVersion        int                              `json:"schemaVersion"`
	Theme                string                           `json:"theme"` // default | steel
	Mode                 string                           `json:"mode"`  // light | dark | system
	LineTotalDecimals    int                              `json:"lineTotalDecimals"`    // 0 | 2
	InvoiceTotalDecimals int                              `json:"invoiceTotalDecimals"` // 0 | 2
	NumberFormat         string                           `json:"numberFormat"`         // en-IN | en-US
	DateFormat           string                           `json:"dateFormat"`           // DD/MM/YYYY | YYYY-MM-DD | MM/DD/YYYY
	TextFormat           string                           `json:"textFormat"`           // preserve | uppercase | titlecase
	Modules              map[string]ModuleDisplaySettings `json:"modules"`
	UpdatedAt            string                           `json:"updatedAt"`
}
