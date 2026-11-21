-- Migration 0017: Multi-Company Profile & Statutory Corporate Entity Details
-- Encrypted per-company metadata table storing legal identity, tax registrations, and bank details.

CREATE TABLE IF NOT EXISTS company_profile (
    id                  TEXT PRIMARY KEY NOT NULL,
    name                TEXT NOT NULL,
    short_code          TEXT NOT NULL UNIQUE,
    iec_code            TEXT NOT NULL DEFAULT '',
    gstin               TEXT NOT NULL DEFAULT '',
    pan                 TEXT NOT NULL DEFAULT '',
    address_line1       TEXT NOT NULL DEFAULT '',
    address_line2       TEXT NOT NULL DEFAULT '',
    city                TEXT NOT NULL DEFAULT '',
    state               TEXT NOT NULL DEFAULT '',
    postal_code         TEXT NOT NULL DEFAULT '',
    country             TEXT NOT NULL DEFAULT 'India',
    phone               TEXT NOT NULL DEFAULT '',
    email               TEXT NOT NULL DEFAULT '',
    website             TEXT NOT NULL DEFAULT '',
    beneficiary_name    TEXT NOT NULL DEFAULT '',
    bank_name           TEXT NOT NULL DEFAULT '',
    account_number      TEXT NOT NULL DEFAULT '',
    ifsc_code           TEXT NOT NULL DEFAULT '',
    swift_code          TEXT NOT NULL DEFAULT '',
    bank_branch         TEXT NOT NULL DEFAULT '',
    bank_address        TEXT NOT NULL DEFAULT '',
    logo_data           TEXT NOT NULL DEFAULT '',
    base_currency       TEXT NOT NULL DEFAULT 'INR',
    financial_year_from TEXT NOT NULL DEFAULT '04-01',
    financial_year_to   TEXT NOT NULL DEFAULT '03-31',
    created_at          TEXT NOT NULL DEFAULT (datetime('now', 'utc')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now', 'utc'))
);

CREATE INDEX IF NOT EXISTS idx_company_profile_short_code ON company_profile(short_code);

-- Seed canonical default primary company profile if absent
INSERT OR IGNORE INTO company_profile (
    id, name, short_code, base_currency, financial_year_from, financial_year_to
) VALUES (
    'comp_default', 'Primary Company', 'PRIMARY', 'INR', '04-01', '03-31'
);
