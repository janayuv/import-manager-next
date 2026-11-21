-- Migration 0019: Add Clearing Agent reimbursement tracking to expense_invoices
-- Allows tracking expenses disbursed by a Clearing Agent (e.g., Liner charges, CFS charges)
-- while preserving the original service provider for GST input tax credit and attributing payment to the CHA.

ALTER TABLE expense_invoices ADD COLUMN is_paid_by_clearing_agent INTEGER DEFAULT 0;
ALTER TABLE expense_invoices ADD COLUMN clearing_agent_id TEXT REFERENCES service_providers(id);
ALTER TABLE expense_invoices ADD COLUMN cha_reimburse_invoice_no TEXT;
ALTER TABLE expense_invoices ADD COLUMN cha_reimburse_invoice_date TEXT;

CREATE INDEX IF NOT EXISTS idx_expense_invoices_clearing_agent_id ON expense_invoices(clearing_agent_id);
CREATE INDEX IF NOT EXISTS idx_expense_invoices_cha_reimb_no ON expense_invoices(cha_reimburse_invoice_no);
