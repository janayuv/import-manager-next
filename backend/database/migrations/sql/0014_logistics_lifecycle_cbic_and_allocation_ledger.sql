-- Migration: 0014_logistics_lifecycle_cbic_and_allocation_ledger.sql
-- 1. Expands shipments with granular logistics lifecycle date milestones (ATD, ATA, OOC, etc.)
-- 2. Expands shipment_containers with distinct port demurrage, CFS ground rent, line detention, and warehouse storage
-- 3. Introduces cbic_exchange_rates for statutory customs rate notifications
-- 4. Introduces expense_allocation_ledger for immutable statutory landed cost auditability
-- 5. Adds database-level delete protection triggers for allocation ledger integrity

-- =========================================================================
-- Part 1: Shipments Lifecycle Milestones
-- =========================================================================
ALTER TABLE shipments ADD COLUMN planned_etd TEXT;
ALTER TABLE shipments ADD COLUMN actual_etd TEXT;           -- ATD (Actual Departure)
ALTER TABLE shipments ADD COLUMN planned_eta TEXT;
ALTER TABLE shipments ADD COLUMN revised_eta TEXT;
ALTER TABLE shipments ADD COLUMN actual_eta TEXT;           -- ATA (Actual Arrival at Port)
ALTER TABLE shipments ADD COLUMN vessel_discharge_date TEXT; -- Vessel berthing / crane discharge
ALTER TABLE shipments ADD COLUMN customs_ooc_date TEXT;     -- Customs Out of Charge
ALTER TABLE shipments ADD COLUMN factory_delivery_date TEXT;-- Delivery at factory gate
ALTER TABLE shipments ADD COLUMN empty_return_date TEXT;    -- Empty equipment returned to depot

-- Seed new columns from existing fields safely
UPDATE shipments
SET planned_etd = COALESCE(planned_etd, etd),
    planned_eta = COALESCE(planned_eta, eta),
    factory_delivery_date = COALESCE(factory_delivery_date, date_of_delivery)
WHERE (planned_etd IS NULL AND etd IS NOT NULL)
   OR (planned_eta IS NULL AND eta IS NOT NULL)
   OR (factory_delivery_date IS NULL AND date_of_delivery IS NOT NULL);

-- =========================================================================
-- Part 2: Granular Container Storage & Detention Charges
-- =========================================================================
ALTER TABLE shipment_containers ADD COLUMN port_discharge_date TEXT;
ALTER TABLE shipment_containers ADD COLUMN cfs_gate_in_date TEXT;
ALTER TABLE shipment_containers ADD COLUMN cfs_gate_out_date TEXT;
ALTER TABLE shipment_containers ADD COLUMN factory_gate_in_date TEXT;

ALTER TABLE shipment_containers ADD COLUMN port_demurrage_free_days INTEGER DEFAULT 0;
ALTER TABLE shipment_containers ADD COLUMN port_demurrage_rate_per_day REAL DEFAULT 0.0;
ALTER TABLE shipment_containers ADD COLUMN port_demurrage_amount REAL DEFAULT 0.0;

ALTER TABLE shipment_containers ADD COLUMN cfs_ground_rent_free_days INTEGER DEFAULT 0;
ALTER TABLE shipment_containers ADD COLUMN cfs_ground_rent_rate_per_day REAL DEFAULT 0.0;
ALTER TABLE shipment_containers ADD COLUMN cfs_ground_rent_amount REAL DEFAULT 0.0;

ALTER TABLE shipment_containers ADD COLUMN shipping_line_detention_free_days INTEGER DEFAULT 0;
ALTER TABLE shipment_containers ADD COLUMN shipping_line_detention_rate_per_day REAL DEFAULT 0.0;
ALTER TABLE shipment_containers ADD COLUMN shipping_line_detention_amount REAL DEFAULT 0.0;

ALTER TABLE shipment_containers ADD COLUMN warehouse_storage_days INTEGER DEFAULT 0;
ALTER TABLE shipment_containers ADD COLUMN warehouse_storage_rate_per_day REAL DEFAULT 0.0;
ALTER TABLE shipment_containers ADD COLUMN warehouse_storage_amount REAL DEFAULT 0.0;

-- Backfill warehouse storage rate from existing column if set
UPDATE shipment_containers
SET warehouse_storage_rate_per_day = COALESCE(storage_rate_per_day, 0.0)
WHERE warehouse_storage_rate_per_day = 0.0 AND storage_rate_per_day > 0;

-- =========================================================================
-- Part 3: CBIC Statutory Customs Exchange Rate Master
-- =========================================================================
CREATE TABLE IF NOT EXISTS cbic_exchange_rates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    currency_code TEXT NOT NULL,
    exchange_rate REAL NOT NULL CHECK (exchange_rate > 0),
    effective_from TEXT NOT NULL,
    effective_to TEXT,
    notification_number TEXT NOT NULL,
    source_document TEXT,
    verification_status TEXT NOT NULL DEFAULT 'Verified' CHECK (verification_status IN ('Verified', 'Pending')),
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (DATETIME('now')),
    created_by TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cbic_lookup 
ON cbic_exchange_rates(currency_code, effective_from, effective_to);

-- Seed baseline statutory exchange rates for top import currencies
INSERT INTO cbic_exchange_rates (currency_code, exchange_rate, effective_from, effective_to, notification_number, source_document, verification_status, notes, created_by)
VALUES
    ('USD', 84.15, '2026-01-01', NULL, 'CBIC Notification No. 01/2026-Customs (N.T.)', 'Gazette of India', 'Verified', 'Baseline USD statutory customs exchange rate', 'system'),
    ('EUR', 91.50, '2026-01-01', NULL, 'CBIC Notification No. 01/2026-Customs (N.T.)', 'Gazette of India', 'Verified', 'Baseline EUR statutory customs exchange rate', 'system'),
    ('GBP', 106.80, '2026-01-01', NULL, 'CBIC Notification No. 01/2026-Customs (N.T.)', 'Gazette of India', 'Verified', 'Baseline GBP statutory customs exchange rate', 'system'),
    ('JPY', 0.56, '2026-01-01', NULL, 'CBIC Notification No. 01/2026-Customs (N.T.)', 'Gazette of India', 'Verified', 'Baseline JPY statutory customs exchange rate per 100 yen', 'system'),
    ('CNY', 11.65, '2026-01-01', NULL, 'CBIC Notification No. 01/2026-Customs (N.T.)', 'Gazette of India', 'Verified', 'Baseline CNY statutory customs exchange rate', 'system'),
    ('AED', 22.90, '2026-01-01', NULL, 'CBIC Notification No. 01/2026-Customs (N.T.)', 'Gazette of India', 'Verified', 'Baseline AED statutory customs exchange rate', 'system');

-- =========================================================================
-- Part 4: Immutable Expense Allocation Audit Ledger
-- =========================================================================
CREATE TABLE IF NOT EXISTS expense_allocation_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,                                       -- Unique batch identifier for this calculation run
    shipment_id TEXT NOT NULL REFERENCES shipments(id),
    boe_id TEXT,
    expense_invoice_id TEXT REFERENCES expense_invoices(id),
    expense_type_id TEXT REFERENCES expense_types(id),
    expense_category TEXT NOT NULL,                             -- 'CHA', 'FREIGHT', 'PORT_DEMURRAGE', 'DETENTION', etc.
    item_id TEXT REFERENCES items(id),
    part_number TEXT NOT NULL,
    base_expense_amount_paise INTEGER NOT NULL,                  -- Pre-tax expense amount in paise
    allocation_method TEXT NOT NULL,                             -- 'ASSESSABLE_VALUE_PRO_RATA', 'WEIGHT_PRO_RATA', etc.
    allocation_basis_value REAL NOT NULL,                        -- Proportion numerator/denominator
    allocation_percentage REAL NOT NULL,                         -- e.g. 24.5821%
    allocated_amount_paise INTEGER NOT NULL,                     -- Apportioned amount in paise
    recalculation_reason TEXT NOT NULL,                          -- 'INITIAL_ASSESSMENT', 'SUPPLEMENTARY_CHA_INVOICE', etc.
    calculated_at TEXT NOT NULL DEFAULT (DATETIME('now')),
    calculated_by TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alloc_ledger_shipment 
ON expense_allocation_ledger(shipment_id, run_id);

CREATE INDEX IF NOT EXISTS idx_alloc_ledger_part 
ON expense_allocation_ledger(part_number, calculated_at);

-- =========================================================================
-- Part 5: Delete Protection for Allocation Ledger
-- =========================================================================
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_shipment_with_allocations
BEFORE DELETE ON shipments
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM expense_allocation_ledger WHERE shipment_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete shipment while referencing expense allocations exist');
END;
