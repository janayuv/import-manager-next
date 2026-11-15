-- Migration 0009: Add insurance_rates lookup table for customizable marine policy rates
CREATE TABLE IF NOT EXISTS insurance_rates (
    value TEXT PRIMARY KEY NOT NULL,
    label TEXT NOT NULL UNIQUE
);

INSERT OR IGNORE INTO insurance_rates (value, label) VALUES
    ('0.015', '0.015% (Open Cover Policy)'),
    ('1.125', '1.125% (Statutory Deemed Rule 10(2))');
