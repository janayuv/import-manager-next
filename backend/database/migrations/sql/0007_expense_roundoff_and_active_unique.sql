-- Migration 0007: Drop hard table-level UNIQUE constraints on expense_invoices and expenses
-- Replace with partial unique indexes WHERE deleted_at IS NULL so soft-deleted records do not block new inserts.
-- Add round_off_paise column to expense_invoices.

PRAGMA foreign_keys = OFF;

-- Temporarily drop report_view which references expense_invoices and expenses
DROP VIEW IF EXISTS report_view;

-- Rebuild expense_invoices
CREATE TABLE expense_invoices_new (
    id TEXT PRIMARY KEY,
    shipment_id TEXT NOT NULL,
    service_provider_id TEXT NOT NULL,
    invoice_no TEXT NOT NULL,
    invoice_date DATE NOT NULL,
    total_amount DECIMAL(12, 2) NOT NULL,
    total_cgst_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    total_sgst_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    total_igst_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    remarks TEXT,
    created_by TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    deleted_by TEXT,
    total_amount_paise INTEGER DEFAULT 0,
    total_cgst_amount_paise INTEGER DEFAULT 0,
    total_sgst_amount_paise INTEGER DEFAULT 0,
    total_igst_amount_paise INTEGER DEFAULT 0,
    total_tds_amount_paise INTEGER DEFAULT 0,
    net_amount_paise INTEGER DEFAULT 0,
    idempotency_key TEXT,
    version INTEGER DEFAULT 1,
    currency TEXT DEFAULT 'INR',
    invoice_number TEXT,
    round_off_paise INTEGER DEFAULT 0,
    FOREIGN KEY (shipment_id) REFERENCES shipments(id),
    FOREIGN KEY (service_provider_id) REFERENCES service_providers(id)
);

INSERT INTO expense_invoices_new (
    id, shipment_id, service_provider_id, invoice_no, invoice_date, total_amount,
    total_cgst_amount, total_sgst_amount, total_igst_amount, remarks,
    created_by, created_at, updated_at, deleted_at, deleted_by,
    total_amount_paise, total_cgst_amount_paise, total_sgst_amount_paise,
    total_igst_amount_paise, total_tds_amount_paise, net_amount_paise,
    idempotency_key, version, currency, invoice_number, round_off_paise
)
SELECT 
    id, shipment_id, service_provider_id, invoice_no, invoice_date, total_amount,
    total_cgst_amount, total_sgst_amount, total_igst_amount, remarks,
    created_by, created_at, updated_at, deleted_at, deleted_by,
    total_amount_paise, total_cgst_amount_paise, total_sgst_amount_paise,
    total_igst_amount_paise, total_tds_amount_paise, net_amount_paise,
    idempotency_key, version, currency, invoice_number, 0
FROM expense_invoices;

DROP TABLE expense_invoices;

ALTER TABLE expense_invoices_new RENAME TO expense_invoices;

CREATE INDEX IF NOT EXISTS idx_expense_invoices_shipment_id ON expense_invoices(shipment_id);
CREATE INDEX IF NOT EXISTS idx_expense_invoices_deleted_at_id ON expense_invoices(deleted_at, id);
CREATE INDEX IF NOT EXISTS idx_expense_invoices_deleted_at ON expense_invoices(deleted_at);
CREATE INDEX IF NOT EXISTS idx_expense_invoices_active_id ON expense_invoices(id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_expense_invoices_active_sp_inv
ON expense_invoices(service_provider_id, invoice_no)
WHERE deleted_at IS NULL;

-- Rebuild expenses
CREATE TABLE expenses_new (
    id TEXT PRIMARY KEY,
    expense_invoice_id TEXT NOT NULL,
    shipment_id TEXT NOT NULL,
    service_provider_id TEXT NOT NULL,
    invoice_no TEXT NOT NULL,
    invoice_date DATE NOT NULL,
    expense_type_id TEXT NOT NULL,
    amount DECIMAL(12, 2) NOT NULL,
    cgst_rate DECIMAL(5, 2) DEFAULT 0.00,
    sgst_rate DECIMAL(5, 2) DEFAULT 0.00,
    igst_rate DECIMAL(5, 2) DEFAULT 0.00,
    tds_rate DECIMAL(5, 2) DEFAULT 0.00,
    cgst_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount * cgst_rate / 100) STORED,
    sgst_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount * sgst_rate / 100) STORED,
    igst_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount * igst_rate / 100) STORED,
    tds_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount * tds_rate / 100) STORED,
    total_amount DECIMAL(12, 2) GENERATED ALWAYS AS (amount + (amount * (cgst_rate + sgst_rate + igst_rate) / 100)) STORED,
    amount_paise INTEGER DEFAULT 0,
    cgst_amount_paise INTEGER DEFAULT 0,
    sgst_amount_paise INTEGER DEFAULT 0,
    igst_amount_paise INTEGER DEFAULT 0,
    tds_amount_paise INTEGER DEFAULT 0,
    total_amount_paise INTEGER DEFAULT 0,
    net_amount_paise INTEGER DEFAULT 0,
    is_manual_gst INTEGER DEFAULT 0,
    remarks TEXT,
    created_by TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    deleted_by TEXT,
    FOREIGN KEY (expense_invoice_id) REFERENCES expense_invoices(id),
    FOREIGN KEY (shipment_id) REFERENCES shipments(id),
    FOREIGN KEY (service_provider_id) REFERENCES service_providers(id),
    FOREIGN KEY (expense_type_id) REFERENCES expense_types(id)
);

INSERT INTO expenses_new (
    id, expense_invoice_id, shipment_id, service_provider_id, invoice_no, invoice_date,
    expense_type_id, amount, cgst_rate, sgst_rate, igst_rate, tds_rate,
    amount_paise, cgst_amount_paise, sgst_amount_paise, igst_amount_paise, tds_amount_paise,
    total_amount_paise, net_amount_paise, is_manual_gst, remarks, created_by, created_at,
    updated_at, deleted_at, deleted_by
)
SELECT 
    id, expense_invoice_id, shipment_id, service_provider_id, invoice_no, invoice_date,
    expense_type_id, amount, cgst_rate, sgst_rate, igst_rate, tds_rate,
    amount_paise, cgst_amount_paise, sgst_amount_paise, igst_amount_paise, tds_amount_paise,
    total_amount_paise, net_amount_paise, is_manual_gst, remarks, created_by, created_at,
    updated_at, deleted_at, deleted_by
FROM expenses;

DROP TABLE expenses;

ALTER TABLE expenses_new RENAME TO expenses;

CREATE INDEX IF NOT EXISTS idx_expenses_shipment_id ON expenses(shipment_id);
CREATE INDEX IF NOT EXISTS idx_expenses_expense_invoice_id ON expenses(expense_invoice_id);
CREATE INDEX IF NOT EXISTS idx_expenses_expense_type_id ON expenses(expense_type_id);
CREATE INDEX IF NOT EXISTS idx_expenses_active_id ON expenses(id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_expenses_active_einv_type
ON expenses(expense_invoice_id, expense_type_id)
WHERE deleted_at IS NULL;

-- Recreate report_view cleanly over the updated tables
CREATE VIEW report_view AS
WITH
boe_calc_items AS (
    SELECT
        bc.id AS boe_calc_id,
        bc.shipment_id,
        bc.supplier_name,
        bc.invoice_number,
        COALESCE(
            NULLIF(json_extract(item.value, '$.partNumber'), ''),
            NULLIF(json_extract(item.value, '$.partNo'), '')
        ) AS part_no,
        json_extract(item.value, '$.description') AS boe_description,
        CAST(COALESCE(json_extract(item.value, '$.assessableValue'), 0.0) AS REAL) AS boe_assessable_value,
        CAST(COALESCE(json_extract(item.value, '$.bcdValue'), 0.0) AS REAL) AS boe_bcd_amount,
        CAST(COALESCE(json_extract(item.value, '$.swsValue'), 0.0) AS REAL) AS boe_sws_amount,
        CAST(COALESCE(json_extract(item.value, '$.igstValue'), 0.0) AS REAL) AS boe_igst_amount
    FROM boe_calculations bc
    JOIN json_each(
        COALESCE(
            json_extract(bc.calculation_result_json, '$.items'),
            json_extract(bc.calculation_result_json, '$.calculatedItems'),
            '[]'
        )
    ) AS item
    WHERE bc.deleted_at IS NULL
),
shipment_expenses AS (
    SELECT
        ei.shipment_id,
        SUM(
            CASE
                WHEN COALESCE(e.amount_paise, 0) > 0 THEN e.amount_paise / 100.0
                ELSE COALESCE(e.amount, 0.0)
            END
        ) AS shipment_expenses_basic,
        SUM(
            CASE
                WHEN COALESCE(e.total_amount_paise, 0) > 0 THEN e.total_amount_paise / 100.0
                ELSE COALESCE(e.amount, 0.0)
            END
        ) AS shipment_expenses_total
    FROM expense_invoices ei
    JOIN expenses e ON e.expense_invoice_id = ei.id AND e.deleted_at IS NULL
    WHERE ei.deleted_at IS NULL
    GROUP BY ei.shipment_id
),
boe_assessable AS (
    SELECT
        shipment_id,
        SUM(boe_assessable_value) AS shipment_boe_assessable_total
    FROM boe_calc_items
    GROUP BY shipment_id
)
SELECT
    sup.supplier_name AS supplier,
    s.supplier_id AS supplier_id,
    s.invoice_number AS invoice_no,
    s.invoice_date AS invoice_date,
    bi.part_no AS part_no,
    COALESCE(NULLIF(i.item_description, ''), bi.boe_description, '') AS description,
    COALESCE(i.unit, '') AS unit,
    COALESCE(ili.quantity, 0.0) AS qty,
    COALESCE(ili.unit_price, 0.0) AS unit_price,
    bi.boe_assessable_value AS assessable_value,
    bi.boe_bcd_amount AS bcd_amount,
    bi.boe_sws_amount AS sws_amount,
    bi.boe_igst_amount AS igst_amount,
    ROUND(
        COALESCE(se.shipment_expenses_basic, 0.0) *
        (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0.0)),
        2
    ) AS expenses_total,
    ROUND(
        (
            bi.boe_assessable_value +
            bi.boe_bcd_amount +
            bi.boe_sws_amount +
            (COALESCE(se.shipment_expenses_basic, 0.0) *
              (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0.0)))
        ) / NULLIF(ili.quantity, 0.0),
        4
    ) AS ldc_per_qty
FROM boe_calc_items bi
JOIN shipments s ON s.id = bi.shipment_id AND s.deleted_at IS NULL
JOIN suppliers sup ON sup.id = s.supplier_id AND sup.deleted_at IS NULL
JOIN invoices inv ON inv.shipment_id = s.id AND inv.deleted_at IS NULL
JOIN items i ON UPPER(i.part_number) = UPPER(bi.part_no) AND i.deleted_at IS NULL
JOIN invoice_line_items ili ON ili.invoice_id = inv.id AND ili.item_id = i.id AND ili.deleted_at IS NULL
LEFT JOIN shipment_expenses se ON se.shipment_id = s.id
LEFT JOIN boe_assessable ba ON ba.shipment_id = s.id;

PRAGMA foreign_keys = ON;
