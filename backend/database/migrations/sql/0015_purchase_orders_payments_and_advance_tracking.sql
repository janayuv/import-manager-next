-- Migration 0015: Purchase Orders, Staged Payment Milestones, Payments, Allocations, and Material Receipts

-- 1. Purchase Orders table
CREATE TABLE IF NOT EXISTS purchase_orders (
    id TEXT PRIMARY KEY NOT NULL,
    po_number TEXT NOT NULL UNIQUE,
    po_date TEXT NOT NULL,
    supplier_id TEXT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USD',
    exchange_rate REAL NOT NULL DEFAULT 1.0,
    incoterm TEXT NOT NULL DEFAULT 'FOB',
    payment_terms_preset TEXT NOT NULL DEFAULT '100_ADVANCE',
    payment_terms_desc TEXT NOT NULL DEFAULT '',
    total_amount REAL NOT NULL DEFAULT 0.0,
    status TEXT NOT NULL DEFAULT 'CONFIRMED' CHECK (status IN ('DRAFT', 'CONFIRMED', 'PARTIALLY_PAID', 'FULLY_PAID', 'COMPLETED', 'CANCELLED')),
    expected_delivery_date TEXT,
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    deleted_by TEXT,
    FOREIGN KEY (supplier_id) REFERENCES suppliers(id)
);

-- 2. PO Line Items table
CREATE TABLE IF NOT EXISTS po_line_items (
    id TEXT PRIMARY KEY NOT NULL,
    po_id TEXT NOT NULL,
    item_id TEXT,
    part_number TEXT NOT NULL,
    item_description TEXT NOT NULL,
    quantity REAL NOT NULL CHECK (quantity > 0),
    unit TEXT NOT NULL DEFAULT 'NOS',
    unit_price REAL NOT NULL CHECK (unit_price >= 0),
    line_total REAL NOT NULL CHECK (line_total >= 0),
    hsn_code TEXT,
    received_quantity REAL NOT NULL DEFAULT 0.0,
    deleted_at TEXT,
    deleted_by TEXT,
    FOREIGN KEY (po_id) REFERENCES purchase_orders(id) ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES items(id)
);

-- 3. PO Payment Milestones table
CREATE TABLE IF NOT EXISTS po_payment_milestones (
    id TEXT PRIMARY KEY NOT NULL,
    po_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    milestone_name TEXT NOT NULL,
    stage_type TEXT NOT NULL DEFAULT 'ADVANCE' CHECK (stage_type IN ('ADVANCE', 'ON_DISPATCH', 'ON_DELIVERY', 'CUSTOM')),
    percentage REAL NOT NULL CHECK (percentage >= 0 AND percentage <= 100),
    amount REAL NOT NULL CHECK (amount >= 0),
    paid_amount REAL NOT NULL DEFAULT 0.0,
    due_date TEXT,
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PARTIALLY_PAID', 'PAID')),
    notes TEXT,
    FOREIGN KEY (po_id) REFERENCES purchase_orders(id) ON DELETE CASCADE
);

-- 4. Payments (Payment Vouchers) table
CREATE TABLE IF NOT EXISTS payments (
    id TEXT PRIMARY KEY NOT NULL,
    voucher_number TEXT NOT NULL UNIQUE,
    payment_date TEXT NOT NULL,
    supplier_id TEXT NOT NULL,
    payment_type TEXT NOT NULL CHECK (payment_type IN ('PO_ADVANCE', 'SINGLE_INVOICE', 'MULTI_INVOICE_BATCH')),
    payment_mode TEXT NOT NULL DEFAULT 'WIRE_TRANSFER' CHECK (payment_mode IN ('WIRE_TRANSFER', 'TT', 'RTGS', 'NEFT', 'LC', 'CHEQUE', 'CASH')),
    currency TEXT NOT NULL DEFAULT 'USD',
    exchange_rate REAL NOT NULL DEFAULT 1.0,
    total_amount REAL NOT NULL CHECK (total_amount >= 0),
    bank_name TEXT NOT NULL DEFAULT '',
    bank_account_no TEXT NOT NULL DEFAULT '',
    reference_number TEXT NOT NULL DEFAULT '',
    beneficiary_name TEXT NOT NULL DEFAULT '',
    beneficiary_bank TEXT NOT NULL DEFAULT '',
    beneficiary_account TEXT NOT NULL DEFAULT '',
    beneficiary_swift TEXT NOT NULL DEFAULT '',
    beneficiary_iban TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'PROCESSED' CHECK (status IN ('DRAFT', 'PROCESSED', 'CANCELLED')),
    notes TEXT,
    created_by TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    deleted_by TEXT,
    FOREIGN KEY (supplier_id) REFERENCES suppliers(id)
);

-- 5. Payment Allocations table
CREATE TABLE IF NOT EXISTS payment_allocations (
    id TEXT PRIMARY KEY NOT NULL,
    payment_id TEXT NOT NULL,
    po_id TEXT,
    po_milestone_id TEXT,
    invoice_id TEXT,
    shipment_id TEXT,
    gross_amount REAL NOT NULL CHECK (gross_amount >= 0),
    advance_adjusted REAL NOT NULL DEFAULT 0.0 CHECK (advance_adjusted >= 0),
    deductions REAL NOT NULL DEFAULT 0.0 CHECK (deductions >= 0),
    net_paid_amount REAL NOT NULL CHECK (net_paid_amount >= 0),
    notes TEXT,
    FOREIGN KEY (payment_id) REFERENCES payments(id) ON DELETE CASCADE,
    FOREIGN KEY (po_id) REFERENCES purchase_orders(id),
    FOREIGN KEY (po_milestone_id) REFERENCES po_payment_milestones(id),
    FOREIGN KEY (invoice_id) REFERENCES invoices(id),
    FOREIGN KEY (shipment_id) REFERENCES shipments(id)
);

-- 6. Material Receipts table
CREATE TABLE IF NOT EXISTS material_receipts (
    id TEXT PRIMARY KEY NOT NULL,
    po_line_item_id TEXT NOT NULL,
    invoice_line_item_id TEXT,
    shipment_id TEXT,
    receipt_date TEXT NOT NULL,
    quantity_received REAL NOT NULL CHECK (quantity_received > 0),
    remarks TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (po_line_item_id) REFERENCES po_line_items(id) ON DELETE CASCADE,
    FOREIGN KEY (invoice_line_item_id) REFERENCES invoice_line_items(id),
    FOREIGN KEY (shipment_id) REFERENCES shipments(id)
);

-- 7. Add po_id columns to shipments and invoices, and po_line_item_id to invoice_line_items for cross-linking
ALTER TABLE shipments ADD COLUMN po_id TEXT;
ALTER TABLE invoices ADD COLUMN po_id TEXT;
ALTER TABLE invoice_line_items ADD COLUMN po_line_item_id TEXT;

-- 8. Indexes for lightning-fast queries
CREATE INDEX IF NOT EXISTS idx_purchase_orders_supplier ON purchase_orders(supplier_id);
CREATE INDEX IF NOT EXISTS idx_purchase_orders_status ON purchase_orders(status);
CREATE INDEX IF NOT EXISTS idx_purchase_orders_deleted ON purchase_orders(deleted_at);
CREATE INDEX IF NOT EXISTS idx_po_line_items_po ON po_line_items(po_id);
CREATE INDEX IF NOT EXISTS idx_po_line_items_item ON po_line_items(item_id);
CREATE INDEX IF NOT EXISTS idx_po_milestones_po ON po_payment_milestones(po_id);
CREATE INDEX IF NOT EXISTS idx_payments_supplier ON payments(supplier_id);
CREATE INDEX IF NOT EXISTS idx_payments_date ON payments(payment_date);
CREATE INDEX IF NOT EXISTS idx_payments_deleted ON payments(deleted_at);
CREATE INDEX IF NOT EXISTS idx_payment_alloc_payment ON payment_allocations(payment_id);
CREATE INDEX IF NOT EXISTS idx_payment_alloc_po ON payment_allocations(po_id);
CREATE INDEX IF NOT EXISTS idx_payment_alloc_invoice ON payment_allocations(invoice_id);
CREATE INDEX IF NOT EXISTS idx_material_receipts_line_item ON material_receipts(po_line_item_id);
CREATE INDEX IF NOT EXISTS idx_shipments_po_id ON shipments(po_id);
CREATE INDEX IF NOT EXISTS idx_invoices_po_id ON invoices(po_id);
