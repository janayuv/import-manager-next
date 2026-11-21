-- Migration 0016: Partial unique indexes for shipments, shipment containers, and suppliers.
-- Resolves existing duplicates safely before creating indexes so migrations never fail on existing data.

PRAGMA foreign_keys = OFF;

-- Step 1: Soft-delete active duplicate shipments that have NO linked dependencies (keep the earliest created record)
UPDATE shipments
SET deleted_at = DATETIME('now'),
    deleted_by = 'migration_0016_dedup'
WHERE deleted_at IS NULL
  AND id NOT IN (
    SELECT MIN(id)
    FROM shipments
    WHERE deleted_at IS NULL
    GROUP BY supplier_id, LOWER(TRIM(invoice_number))
  )
  AND id NOT IN (
    SELECT DISTINCT shipment_id FROM invoices WHERE deleted_at IS NULL
    UNION
    SELECT DISTINCT shipment_id FROM boe_calculations WHERE deleted_at IS NULL
    UNION
    SELECT DISTINCT shipment_id FROM expense_invoices WHERE deleted_at IS NULL
    UNION
    SELECT DISTINCT shipment_id FROM shipment_containers WHERE deleted_at IS NULL
  );

-- Step 2: Disambiguate any remaining active duplicate shipments that DO have dependencies by suffixing invoice number
UPDATE shipments
SET invoice_number = invoice_number || '-DUP-' || id
WHERE deleted_at IS NULL
  AND id NOT IN (
    SELECT MIN(id)
    FROM shipments
    WHERE deleted_at IS NULL
    GROUP BY supplier_id, LOWER(TRIM(invoice_number))
  );

-- Step 3: Enforce partial unique index on active shipments per (supplier_id, invoice_number)
CREATE UNIQUE INDEX IF NOT EXISTS idx_shipments_active_supplier_invoice
ON shipments(supplier_id, invoice_number COLLATE NOCASE)
WHERE deleted_at IS NULL;

-- Step 4: Disambiguate active duplicate containers within the same shipment before index creation
UPDATE shipment_containers
SET deleted_at = DATETIME('now')
WHERE deleted_at IS NULL
  AND id NOT IN (
    SELECT MIN(id)
    FROM shipment_containers
    WHERE deleted_at IS NULL
    GROUP BY shipment_id, LOWER(TRIM(container_number))
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_shipment_containers_active_shipment_cont
ON shipment_containers(shipment_id, container_number COLLATE NOCASE)
WHERE deleted_at IS NULL;

-- Step 5: Enforce partial unique index on active suppliers
CREATE UNIQUE INDEX IF NOT EXISTS idx_suppliers_active_name
ON suppliers(supplier_name COLLATE NOCASE)
WHERE deleted_at IS NULL;

PRAGMA foreign_keys = ON;
