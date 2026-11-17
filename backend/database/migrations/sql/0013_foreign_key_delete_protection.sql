-- Migration: 0013_foreign_key_delete_protection.sql
-- Establishes database-level delete protection triggers across core business entities,
-- preventing hard-deletion of parent records whenever referencing child records exist.
-- This neutralizes accidental CASCADE destruction of critical financial and customs records.

-- 1. Shipments Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_shipment_with_invoices
BEFORE DELETE ON shipments
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM invoices WHERE shipment_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete shipment while referencing invoices exist');
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_shipment_with_boe_calcs
BEFORE DELETE ON shipments
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM boe_calculations WHERE shipment_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete shipment while referencing BOE calculations exist');
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_shipment_with_containers
BEFORE DELETE ON shipments
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM shipment_containers WHERE shipment_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete shipment while referencing containers exist');
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_shipment_with_expense_invoices
BEFORE DELETE ON shipments
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM expense_invoices WHERE shipment_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete shipment while referencing expense invoices exist');
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_shipment_with_expenses
BEFORE DELETE ON shipments
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM expenses WHERE shipment_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete shipment while referencing expenses exist');
END;

-- 2. Invoices Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_invoice_with_line_items
BEFORE DELETE ON invoices
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM invoice_line_items WHERE invoice_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete invoice while referencing invoice line items exist');
END;

-- 3. BOE Calculations Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_boe_calc_with_items
BEFORE DELETE ON boe_calculations
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM boe_items WHERE boe_calculation_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete BOE calculation while referencing BOE items exist');
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_boe_calc_with_attachments
BEFORE DELETE ON boe_calculations
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM boe_attachments WHERE boe_calculation_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete BOE calculation while referencing attachments exist');
END;

-- 4. BOE Details Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_boe_detail_with_calcs
BEFORE DELETE ON boe_details
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM boe_calculations WHERE boe_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete BOE details while referencing BOE calculations exist');
END;

-- 5. Items Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_item_with_invoice_line_items
BEFORE DELETE ON items
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM invoice_line_items WHERE item_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete item while referencing invoice line items exist');
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_item_with_boe_items
BEFORE DELETE ON items
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM boe_items WHERE item_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete item while referencing BOE items exist');
END;

-- 6. Suppliers Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_supplier_with_shipments
BEFORE DELETE ON suppliers
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM shipments WHERE supplier_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete supplier while referencing shipments exist');
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_supplier_with_items
BEFORE DELETE ON suppliers
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM items WHERE supplier_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete supplier while referencing items exist');
END;

-- 7. Service Providers Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_service_provider_with_expense_invoices
BEFORE DELETE ON service_providers
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM expense_invoices WHERE service_provider_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete service provider while referencing expense invoices exist');
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_service_provider_with_expenses
BEFORE DELETE ON service_providers
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM expenses WHERE service_provider_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete service provider while referencing expenses exist');
END;

-- 8. Expense Types Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_expense_type_with_expenses
BEFORE DELETE ON expense_types
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM expenses WHERE expense_type_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete expense type while referencing expenses exist');
END;

-- 9. Expense Invoices Deletion Protection
CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_expense_invoice_with_expenses
BEFORE DELETE ON expense_invoices
FOR EACH ROW
WHEN (SELECT COUNT(*) FROM expenses WHERE expense_invoice_id = OLD.id) > 0
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY RESTRICTION: cannot delete expense invoice while referencing expenses exist');
END;
