-- Migration 0010: Drop hard table-level UNIQUE constraint on boe_details(be_number, be_date)
-- Replace with partial unique index WHERE deleted_at IS NULL so soft-deleted records do not block new inserts or CSV re-imports.

PRAGMA foreign_keys = OFF;

CREATE TABLE boe_details_new (
    id TEXT PRIMARY KEY NOT NULL,
    be_number TEXT NOT NULL,
    be_date TEXT NOT NULL,
    location TEXT NOT NULL,
    total_assessment_value REAL NOT NULL,
    duty_amount REAL NOT NULL,
    payment_date TEXT,
    duty_paid REAL,
    challan_number TEXT,
    ref_id TEXT,
    transaction_id TEXT,
    deleted_at TEXT,
    deleted_by TEXT
);

INSERT INTO boe_details_new (
    id, be_number, be_date, location, total_assessment_value, duty_amount,
    payment_date, duty_paid, challan_number, ref_id, transaction_id,
    deleted_at, deleted_by
)
SELECT 
    id, be_number, be_date, location, total_assessment_value, duty_amount,
    payment_date, duty_paid, challan_number, ref_id, transaction_id,
    deleted_at, deleted_by
FROM boe_details;

DROP TABLE boe_details;

ALTER TABLE boe_details_new RENAME TO boe_details;

CREATE INDEX IF NOT EXISTS idx_boe_details_deleted_at_id ON boe_details(deleted_at, id);
CREATE INDEX IF NOT EXISTS idx_boe_details_deleted_at ON boe_details(deleted_at);
CREATE INDEX IF NOT EXISTS idx_boe_details_active_id ON boe_details(id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_boe_number ON boe_details(be_number);

CREATE UNIQUE INDEX IF NOT EXISTS idx_boe_details_active_be_num_date
ON boe_details(be_number, be_date)
WHERE deleted_at IS NULL;

PRAGMA foreign_keys = ON;
