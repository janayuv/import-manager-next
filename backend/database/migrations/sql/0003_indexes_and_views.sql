-- === V52__supplier_pagination_indexes.sql ===
CREATE INDEX IF NOT EXISTS idx_suppliers_name_active
ON suppliers(supplier_name)
WHERE deleted_at IS NULL;

-- === V55__db_management_performance_indexes.sql ===
-- Database-management performance indexes (safe, no behavior change).

-- Audit query acceleration for filtered + ordered log views.
CREATE INDEX IF NOT EXISTS idx_audit_logs_table_action_created_at
ON audit_logs("tableName", action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created_at
ON audit_logs(user_id, created_at DESC);

-- Recycle-bin access path (deleted rows sorted by deleted_at).
CREATE INDEX IF NOT EXISTS idx_suppliers_deleted_at_id
ON suppliers(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_shipments_deleted_at_id
ON shipments(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_items_deleted_at_id
ON items(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_invoices_deleted_at_id
ON invoices(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_boe_details_deleted_at_id
ON boe_details(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_boe_calculations_deleted_at_id
ON boe_calculations(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_service_providers_deleted_at_id
ON service_providers(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_expense_types_deleted_at_id
ON expense_types(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_expense_invoices_deleted_at_id
ON expense_invoices(deleted_at, id)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_expenses_deleted_at_id
ON expenses(deleted_at, id)
WHERE deleted_at IS NOT NULL;

-- === Soft-delete & active-row partial indexes (from legacy post_refinery_migrations) ===
CREATE INDEX IF NOT EXISTS idx_suppliers_deleted_at ON suppliers(deleted_at);
CREATE INDEX IF NOT EXISTS idx_shipments_deleted_at ON shipments(deleted_at);
CREATE INDEX IF NOT EXISTS idx_items_deleted_at ON items(deleted_at);
CREATE INDEX IF NOT EXISTS idx_invoices_deleted_at ON invoices(deleted_at);
CREATE INDEX IF NOT EXISTS idx_boe_details_deleted_at ON boe_details(deleted_at);
CREATE INDEX IF NOT EXISTS idx_boe_calculations_deleted_at ON boe_calculations(deleted_at);
CREATE INDEX IF NOT EXISTS idx_service_providers_deleted_at ON service_providers(deleted_at);
CREATE INDEX IF NOT EXISTS idx_expense_types_deleted_at ON expense_types(deleted_at);
CREATE INDEX IF NOT EXISTS idx_expense_invoices_deleted_at ON expense_invoices(deleted_at);
CREATE INDEX IF NOT EXISTS idx_expenses_deleted_at ON expenses(deleted_at);
CREATE INDEX IF NOT EXISTS idx_workflow_incidents_deleted_at ON workflow_incidents(deleted_at);
CREATE INDEX IF NOT EXISTS idx_suppliers_active_id ON suppliers(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_shipments_active_id ON shipments(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_items_active_id ON items(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invoices_active_id ON invoices(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_boe_details_active_id ON boe_details(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_boe_calculations_active_id ON boe_calculations(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_service_providers_active_id ON service_providers(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_expense_types_active_id ON expense_types(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_expense_invoices_active_id ON expense_invoices(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_expenses_active_id ON expenses(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflow_incidents_active_id ON workflow_incidents(incident_id) WHERE deleted_at IS NULL;

-- === V80__backfill_expense_paise_from_decimal.sql ===
-- Backfill *_paise columns on the expenses table for legacy rows.
--
-- When the paise columns were added via ALTER TABLE ... DEFAULT 0, SQLite
-- filled every existing row with 0 (not NULL). The subsequent migration
-- backfill used WHERE amount_paise IS NULL, which matched nothing, leaving
-- legacy rows with zeroed paise columns while the decimal columns (amount,
-- cgst_amount, …) still hold the real values.
--
-- This migration corrects those rows: any row where amount_paise = 0 but
-- amount > 0 is treated as a legacy row that was never backfilled.

UPDATE expenses
SET
    amount_paise       = CAST(ROUND(amount                        * 100) AS INTEGER),
    cgst_amount_paise  = CAST(ROUND(cgst_amount                   * 100) AS INTEGER),
    sgst_amount_paise  = CAST(ROUND(sgst_amount                   * 100) AS INTEGER),
    igst_amount_paise  = CAST(ROUND(igst_amount                   * 100) AS INTEGER),
    tds_amount_paise   = CAST(ROUND(tds_amount                    * 100) AS INTEGER),
    total_amount_paise = CAST(ROUND(total_amount                  * 100) AS INTEGER),
    -- net_amount was never added as a decimal column on upgraded installs (only
    -- net_amount_paise INTEGER was added via ALTER TABLE). Derive net from the
    -- GENERATED ALWAYS columns that exist in the original V1 schema.
    net_amount_paise   = CAST(ROUND((total_amount - tds_amount)   * 100) AS INTEGER)
WHERE amount_paise = 0
  AND amount IS NOT NULL
  AND amount != 0;

-- Same correction for expense_invoices summary totals.
-- Note: total_tds_amount (decimal) was never added to expense_invoices; only
-- total_tds_amount_paise (INTEGER) exists. Legacy invoices carried no TDS so
-- total_tds_amount_paise stays 0 and net equals gross.
UPDATE expense_invoices
SET
    total_amount_paise      = CAST(ROUND(total_amount      * 100) AS INTEGER),
    total_cgst_amount_paise = CAST(ROUND(total_cgst_amount * 100) AS INTEGER),
    total_sgst_amount_paise = CAST(ROUND(total_sgst_amount * 100) AS INTEGER),
    total_igst_amount_paise = CAST(ROUND(total_igst_amount * 100) AS INTEGER),
    total_tds_amount_paise  = 0,
    net_amount_paise        = CAST(ROUND(total_amount      * 100) AS INTEGER)
WHERE total_amount_paise = 0
  AND total_amount IS NOT NULL
  AND total_amount != 0;

-- === Authoritative Landed Cost Report View ===
DROP VIEW IF EXISTS report_view;
CREATE VIEW IF NOT EXISTS report_view AS
WITH
boe_items_cte AS (
    SELECT
        bc.id AS boe_calc_id,
        bc.shipment_id,
        bc.supplier_name,
        bc.invoice_number,
        json_extract(item.value, '$.partNo') AS part_no,
        json_extract(item.value, '$.description') AS boe_description,
        CAST(json_extract(item.value, '$.assessableValue') AS REAL) AS boe_assessable_value,
        CAST(json_extract(item.value, '$.bcdValue') AS REAL) AS boe_bcd_amount,
        CAST(json_extract(item.value, '$.swsValue') AS REAL) AS boe_sws_amount,
        CAST(json_extract(item.value, '$.igstValue') AS REAL) AS boe_igst_amount
    FROM boe_calculations bc
    JOIN json_each(json_extract(bc.calculation_result_json, '$.calculatedItems')) AS item
    WHERE bc.deleted_at IS NULL
),
shipment_expenses AS (
    SELECT
        ei.shipment_id,
        SUM(e.amount) AS shipment_expenses_basic,
        SUM(e.total_amount) AS shipment_expenses_total
    FROM expense_invoices ei
    JOIN expenses e ON e.expense_invoice_id = ei.id
    WHERE ei.deleted_at IS NULL AND e.deleted_at IS NULL
    GROUP BY ei.shipment_id
),
boe_assessable AS (
    SELECT shipment_id, SUM(boe_assessable_value) AS shipment_boe_assessable_total
    FROM boe_items_cte
    GROUP BY shipment_id
)
SELECT
    sup.supplier_name AS supplier,
    s.supplier_id AS supplier_id,
    s.invoice_number AS invoice_no,
    s.invoice_date AS invoice_date,
    bi.part_no AS part_no,
    COALESCE(i.item_description, bi.boe_description) AS description,
    i.unit AS unit,
    ili.quantity AS qty,
    ili.unit_price AS unit_price,
    bi.boe_assessable_value AS assessable_value,
    bi.boe_bcd_amount AS bcd_amount,
    bi.boe_sws_amount AS sws_amount,
    bi.boe_igst_amount AS igst_amount,
    COALESCE(se.shipment_expenses_basic, 0.0) *
      (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0)) AS expenses_total,
    (
      (bi.boe_assessable_value + bi.boe_bcd_amount + bi.boe_sws_amount
       + (COALESCE(se.shipment_expenses_basic, 0.0) * (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0))))
    ) / NULLIF(ili.quantity, 0) AS ldc_per_qty
FROM boe_items_cte bi
JOIN shipments s ON s.id = bi.shipment_id AND s.deleted_at IS NULL
JOIN suppliers sup ON sup.id = s.supplier_id AND sup.deleted_at IS NULL
JOIN invoices inv ON inv.shipment_id = s.id AND inv.deleted_at IS NULL
JOIN items i ON i.part_number = bi.part_no AND i.deleted_at IS NULL
JOIN invoice_line_items ili ON ili.invoice_id = inv.id AND ili.item_id = i.id AND ili.deleted_at IS NULL
LEFT JOIN shipment_expenses se ON se.shipment_id = s.id
LEFT JOIN boe_assessable ba ON ba.shipment_id = s.id;
