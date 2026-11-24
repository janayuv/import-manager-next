-- Migration 0020: RoDTEP Scrip Financial Sub-Ledger, Manual Allocation & Append-Only Audit Trail
-- Integer paise storage throughout (1 INR = 100 paise).

PRAGMA foreign_keys = ON;

-- 1. RoDTEP Scrips Master Table
CREATE TABLE IF NOT EXISTS rodtep_scrips (
    id TEXT PRIMARY KEY NOT NULL,
    scrip_number TEXT NOT NULL COLLATE NOCASE,
    scrip_port_code TEXT NOT NULL CHECK (scrip_port_code IN ('INMAA1', 'INMAA4')), -- Registration Port
    acquisition_type TEXT NOT NULL DEFAULT 'DIRECT_GENERATION' CHECK (acquisition_type IN ('DIRECT_GENERATION', 'OPENING_BALANCE')),
    issue_date TEXT NOT NULL CHECK (length(issue_date) = 10),                     -- YYYY-MM-DD
    expiry_date TEXT NOT NULL CHECK (length(expiry_date) = 10),                    -- YYYY-MM-DD (Explicit Statutory Date)
    original_value_paise INTEGER NOT NULL CHECK (original_value_paise > 0),
    opening_balance_paise INTEGER NOT NULL CHECK (opening_balance_paise >= 0),
    pre_erp_utilized_paise INTEGER NOT NULL DEFAULT 0 CHECK (pre_erp_utilized_paise >= 0),
    confirmed_utilized_paise INTEGER NOT NULL DEFAULT 0 CHECK (confirmed_utilized_paise >= 0),
    reversal_recredit_paise INTEGER NOT NULL DEFAULT 0 CHECK (reversal_recredit_paise >= 0),
    confirmed_balance_paise INTEGER NOT NULL CHECK (confirmed_balance_paise >= 0),
    pending_allocated_paise INTEGER NOT NULL DEFAULT 0 CHECK (pending_allocated_paise >= 0),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'EXHAUSTED', 'EXPIRED', 'SUSPENDED')),
    icegate_user_id TEXT,
    registration_details TEXT,
    document_ref TEXT,
    remarks TEXT,
    opening_verified_at TEXT,
    opening_verified_by TEXT,
    opening_evidence_ref TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    deleted_at TEXT,
    deleted_by TEXT,
    -- Financial Movement Conservation Invariant
    CHECK (confirmed_balance_paise = opening_balance_paise - confirmed_utilized_paise + reversal_recredit_paise),
    -- Encumbrance Protection Invariant
    CHECK (confirmed_balance_paise >= pending_allocated_paise)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rodtep_scrips_active_number
ON rodtep_scrips(scrip_number)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_rodtep_scrips_port_status 
ON rodtep_scrips(scrip_port_code, status) 
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_rodtep_scrips_expiry 
ON rodtep_scrips(expiry_date) 
WHERE deleted_at IS NULL;

-- 2. RoDTEP Internal BOE Allocations Table (Stage 1: Intent & Reservation)
CREATE TABLE IF NOT EXISTS rodtep_allocations (
    id TEXT PRIMARY KEY NOT NULL,
    allocation_ref TEXT NOT NULL UNIQUE,                                -- e.g. ALC-20261124-001
    scrip_id TEXT NOT NULL,
    boe_id TEXT NOT NULL,                                              -- References boe_details(id)
    boe_number TEXT NOT NULL,
    boe_date TEXT NOT NULL,
    boe_port_code TEXT NOT NULL,                                        -- Port of Import
    allocated_amount_paise INTEGER NOT NULL CHECK (allocated_amount_paise > 0),
    confirmed_amount_paise INTEGER NOT NULL DEFAULT 0 CHECK (confirmed_amount_paise >= 0),
    released_amount_paise INTEGER NOT NULL DEFAULT 0 CHECK (released_amount_paise >= 0),
    status TEXT NOT NULL DEFAULT 'ALLOCATED' CHECK (status IN ('DRAFT', 'ALLOCATED', 'PARTIALLY_CONFIRMED', 'CONFIRMED', 'CANCELLED', 'CLOSED')),
    operator_user_id TEXT NOT NULL,
    operator_name TEXT NOT NULL,
    remarks TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (scrip_id) REFERENCES rodtep_scrips(id) ON DELETE RESTRICT,
    FOREIGN KEY (boe_id) REFERENCES boe_details(id) ON DELETE RESTRICT,
    CHECK (confirmed_amount_paise + released_amount_paise <= allocated_amount_paise)
);

CREATE INDEX IF NOT EXISTS idx_rodtep_alloc_scrip_status ON rodtep_allocations(scrip_id, status);
CREATE INDEX IF NOT EXISTS idx_rodtep_alloc_boe_id ON rodtep_allocations(boe_id);

-- 3. RoDTEP Append-Only Statutory Debit Ledger (Stage 2: Actual Utilization & Reversals)
CREATE TABLE IF NOT EXISTS rodtep_debit_ledger (
    id TEXT PRIMARY KEY NOT NULL,
    transaction_ref TEXT NOT NULL UNIQUE,                                -- e.g. TX-20261124-001
    allocation_id TEXT,                                                 -- Link to allocation
    scrip_id TEXT NOT NULL,
    scrip_number TEXT NOT NULL,
    scrip_port_code TEXT NOT NULL CHECK (scrip_port_code IN ('INMAA1', 'INMAA4')),
    boe_id TEXT NOT NULL,                                              -- References boe_details(id)
    boe_number TEXT NOT NULL,
    boe_date TEXT NOT NULL,
    boe_port_code TEXT NOT NULL,                                        -- Port of Import
    tx_type TEXT NOT NULL CHECK (tx_type IN ('DEBIT_CONFIRMED', 'DEBIT_REVERSAL')),
    amount_paise INTEGER NOT NULL CHECK (amount_paise > 0),             -- Absolute amount in paise
    previous_confirmed_balance_paise INTEGER NOT NULL CHECK (previous_confirmed_balance_paise >= 0),
    remaining_confirmed_balance_paise INTEGER NOT NULL CHECK (remaining_confirmed_balance_paise >= 0),
    icegate_transaction_ref TEXT,                                       -- Nullable on internal reversals
    challan_number TEXT,
    debit_date TEXT NOT NULL,                                           -- Date of actual ICEGATE debit
    operator_user_id TEXT NOT NULL,
    operator_name TEXT NOT NULL,
    supporting_doc_ref TEXT,
    remarks TEXT,
    reversal_of_id TEXT,                                                -- References rodtep_debit_ledger(id) on reversal
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (allocation_id) REFERENCES rodtep_allocations(id) ON DELETE RESTRICT,
    FOREIGN KEY (scrip_id) REFERENCES rodtep_scrips(id) ON DELETE RESTRICT,
    FOREIGN KEY (boe_id) REFERENCES boe_details(id) ON DELETE RESTRICT,
    FOREIGN KEY (reversal_of_id) REFERENCES rodtep_debit_ledger(id) ON DELETE RESTRICT,
    -- ICEGATE Evidence Invariant: Confirmed debits must have icegate ref or supporting doc ref
    CHECK (
        tx_type != 'DEBIT_CONFIRMED' OR 
        (icegate_transaction_ref IS NOT NULL AND length(trim(icegate_transaction_ref)) > 0) OR
        (supporting_doc_ref IS NOT NULL AND length(trim(supporting_doc_ref)) > 0)
    )
);

-- Partial Unique Index: Non-null ICEGATE transaction references must be unique
CREATE UNIQUE INDEX IF NOT EXISTS idx_rodtep_ledger_icegate_ref
ON rodtep_debit_ledger(icegate_transaction_ref)
WHERE icegate_transaction_ref IS NOT NULL AND icegate_transaction_ref != '';

CREATE INDEX IF NOT EXISTS idx_rodtep_ledger_scrip_date ON rodtep_debit_ledger(scrip_id, debit_date);
CREATE INDEX IF NOT EXISTS idx_rodtep_ledger_boe_id ON rodtep_debit_ledger(boe_id);
CREATE INDEX IF NOT EXISTS idx_rodtep_ledger_reversal ON rodtep_debit_ledger(reversal_of_id);

-- 4. RoDTEP Universal Audit & Governance Log (All Foreign Keys ON DELETE RESTRICT)
CREATE TABLE IF NOT EXISTS rodtep_audit_logs (
    id TEXT PRIMARY KEY NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'SCRIP_CREATED', 'SCRIP_UPDATED', 'SCRIP_STATUS_CHANGE',
        'ALLOCATION_RESERVED', 'ALLOCATION_RELEASED',
        'DEBIT_CONFIRMED', 'DEBIT_REVERSED',
        'RECONCILIATION_ENTRY'
    )),
    scrip_id TEXT NOT NULL,
    allocation_id TEXT,
    ledger_id TEXT,
    boe_id TEXT,
    amount_paise INTEGER,
    old_values_json TEXT,
    new_values_json TEXT,
    reason TEXT NOT NULL,
    performed_by TEXT NOT NULL,
    performed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (scrip_id) REFERENCES rodtep_scrips(id) ON DELETE RESTRICT,
    FOREIGN KEY (allocation_id) REFERENCES rodtep_allocations(id) ON DELETE RESTRICT,
    FOREIGN KEY (ledger_id) REFERENCES rodtep_debit_ledger(id) ON DELETE RESTRICT,
    FOREIGN KEY (boe_id) REFERENCES boe_details(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_rodtep_audit_scrip ON rodtep_audit_logs(scrip_id, performed_at);
CREATE INDEX IF NOT EXISTS idx_rodtep_audit_event ON rodtep_audit_logs(event_type);

-- 5. ICEGATE Monthly Statement Reconciliation Table
CREATE TABLE IF NOT EXISTS rodtep_reconciliations (
    id TEXT PRIMARY KEY NOT NULL,
    scrip_id TEXT NOT NULL,
    reconciliation_date TEXT NOT NULL,                                  -- YYYY-MM-DD
    statement_period_end TEXT NOT NULL,                                 -- YYYY-MM-DD
    icegate_statement_balance_paise INTEGER NOT NULL CHECK (icegate_statement_balance_paise >= 0),
    erp_confirmed_balance_paise INTEGER NOT NULL CHECK (erp_confirmed_balance_paise >= 0),
    erp_pending_allocated_paise INTEGER NOT NULL DEFAULT 0,
    discrepancy_paise INTEGER NOT NULL,                                 -- (icegate_balance - erp_confirmed_balance)
    discrepancy_reason TEXT,
    supporting_statement_path TEXT,
    reconciled_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    FOREIGN KEY (scrip_id) REFERENCES rodtep_scrips(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_rodtep_reconcile_scrip ON rodtep_reconciliations(scrip_id, reconciliation_date);

-- =========================================================================
-- Part 6: Database-Level Immutability & Reversal Validation Triggers
-- =========================================================================

-- Reversal Limits & Referential Validation
CREATE TRIGGER IF NOT EXISTS trg_validate_reversal_limits
BEFORE INSERT ON rodtep_debit_ledger
FOR EACH ROW
WHEN NEW.tx_type = 'DEBIT_REVERSAL'
BEGIN
    SELECT CASE
        WHEN NEW.reversal_of_id IS NULL THEN
            RAISE(ABORT, 'REVERSAL VALIDATION: reversal_of_id is required for DEBIT_REVERSAL')
        WHEN (SELECT COUNT(*) FROM rodtep_debit_ledger WHERE id = NEW.reversal_of_id AND tx_type = 'DEBIT_CONFIRMED') = 0 THEN
            RAISE(ABORT, 'REVERSAL VALIDATION: referenced transaction must be an existing DEBIT_CONFIRMED record')
        WHEN (SELECT scrip_id FROM rodtep_debit_ledger WHERE id = NEW.reversal_of_id) != NEW.scrip_id THEN
            RAISE(ABORT, 'REVERSAL VALIDATION: reversal scrip_id must match the original debit scrip_id')
        WHEN (SELECT boe_id FROM rodtep_debit_ledger WHERE id = NEW.reversal_of_id) != NEW.boe_id THEN
            RAISE(ABORT, 'REVERSAL VALIDATION: reversal boe_id must match the original debit boe_id')
        WHEN (
            COALESCE((SELECT SUM(amount_paise) FROM rodtep_debit_ledger WHERE reversal_of_id = NEW.reversal_of_id), 0) + NEW.amount_paise
        ) > (SELECT amount_paise FROM rodtep_debit_ledger WHERE id = NEW.reversal_of_id) THEN
            RAISE(ABORT, 'REVERSAL VALIDATION: cumulative reversal amount exceeds the original debit amount')
    END;
END;

-- Prevent UPDATE on rodtep_debit_ledger
CREATE TRIGGER IF NOT EXISTS trg_prevent_update_rodtep_debit_ledger
BEFORE UPDATE ON rodtep_debit_ledger
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'IMMUTABLE LEDGER RESTRICTION: updates to rodtep_debit_ledger are strictly prohibited; use append-only reversal records.');
END;

-- Prevent DELETE on rodtep_debit_ledger
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_rodtep_debit_ledger
BEFORE DELETE ON rodtep_debit_ledger
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'IMMUTABLE LEDGER RESTRICTION: deletions from rodtep_debit_ledger are strictly prohibited.');
END;

-- Prevent Tampering with Original Allocation Parameters
CREATE TRIGGER IF NOT EXISTS trg_prevent_tamper_rodtep_allocations
BEFORE UPDATE ON rodtep_allocations
FOR EACH ROW
BEGIN
    SELECT CASE
        WHEN OLD.allocated_amount_paise != NEW.allocated_amount_paise THEN
            RAISE(ABORT, 'IMMUTABLE ALLOCATION: allocated_amount_paise cannot be modified; use release or cancellation')
        WHEN OLD.scrip_id != NEW.scrip_id THEN
            RAISE(ABORT, 'IMMUTABLE ALLOCATION: scrip_id cannot be changed')
        WHEN OLD.boe_id != NEW.boe_id THEN
            RAISE(ABORT, 'IMMUTABLE ALLOCATION: boe_id cannot be changed')
        WHEN OLD.allocation_ref != NEW.allocation_ref THEN
            RAISE(ABORT, 'IMMUTABLE ALLOCATION: allocation_ref cannot be changed')
    END;
END;

-- Prevent DELETE on rodtep_allocations
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_rodtep_allocations
BEFORE DELETE ON rodtep_allocations
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'IMMUTABLE ALLOCATION: deletions from rodtep_allocations are strictly prohibited; update status to CANCELLED or CLOSED.');
END;

-- Prevent UPDATE on rodtep_audit_logs
CREATE TRIGGER IF NOT EXISTS trg_prevent_update_rodtep_audit_logs
BEFORE UPDATE ON rodtep_audit_logs
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'IMMUTABLE AUDIT RESTRICTION: updates to rodtep_audit_logs are strictly prohibited.');
END;

-- Prevent DELETE on rodtep_audit_logs
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_rodtep_audit_logs
BEFORE DELETE ON rodtep_audit_logs
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'IMMUTABLE AUDIT RESTRICTION: deletions from rodtep_audit_logs are strictly prohibited.');
END;

-- Prevent UPDATE on rodtep_reconciliations
CREATE TRIGGER IF NOT EXISTS trg_prevent_update_rodtep_reconciliations
BEFORE UPDATE ON rodtep_reconciliations
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'IMMUTABLE RECONCILIATION: updates to rodtep_reconciliations are strictly prohibited; create a new reconciliation record.');
END;

-- Prevent DELETE on rodtep_reconciliations
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_rodtep_reconciliations
BEFORE DELETE ON rodtep_reconciliations
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'IMMUTABLE RECONCILIATION: deletions from rodtep_reconciliations are strictly prohibited.');
END;
