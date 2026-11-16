-- Migration 0012: Multi-container tracking, warehouse storage, staged delivery, and dwell time calculation

CREATE TABLE IF NOT EXISTS shipment_containers (
    id TEXT PRIMARY KEY,
    shipment_id TEXT NOT NULL,
    container_number TEXT NOT NULL,
    container_size_type TEXT NOT NULL DEFAULT '40FT',
    seal_number TEXT,
    route_type TEXT NOT NULL DEFAULT 'DIRECT', -- 'DIRECT' or 'WAREHOUSE'
    warehouse_name TEXT,
    gate_in_date TEXT,
    gate_out_date TEXT,
    free_days_allowed INTEGER NOT NULL DEFAULT 0,
    storage_rate_per_day REAL NOT NULL DEFAULT 0.0,
    delivery_status TEXT NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'IN_TRANSIT', 'IN_WAREHOUSE', 'DELIVERED'
    date_of_delivery TEXT,
    transporter_name TEXT,
    vehicle_number TEXT,
    lr_number TEXT,
    empty_return_date TEXT,
    remarks TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    deleted_at TEXT,
    FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_shipment_containers_shipment_id ON shipment_containers(shipment_id);
CREATE INDEX IF NOT EXISTS idx_shipment_containers_status ON shipment_containers(delivery_status);
CREATE INDEX IF NOT EXISTS idx_shipment_containers_wh_in ON shipment_containers(warehouse_name, gate_in_date);

-- Backfill existing shipments that have a container_number into shipment_containers
INSERT INTO shipment_containers (
    id,
    shipment_id,
    container_number,
    container_size_type,
    route_type,
    delivery_status,
    date_of_delivery,
    created_at,
    updated_at
)
SELECT
    'CONT-' || s.id || '-01',
    s.id,
    s.container_number,
    CASE 
        WHEN UPPER(COALESCE(s.shipment_type, '')) LIKE '%LCL%' THEN 'LCL'
        ELSE '40FT'
    END,
    'DIRECT',
    CASE
        WHEN LOWER(COALESCE(s.status, '')) = 'delivered' OR (s.date_of_delivery IS NOT NULL AND TRIM(s.date_of_delivery) != '') THEN 'DELIVERED'
        ELSE 'PENDING'
    END,
    s.date_of_delivery,
    COALESCE(s.created_at, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
    COALESCE(s.created_at, strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime'))
FROM shipments s
WHERE s.deleted_at IS NULL
  AND s.container_number IS NOT NULL
  AND TRIM(s.container_number) != ''
  AND NOT EXISTS (
      SELECT 1 FROM shipment_containers sc WHERE sc.shipment_id = s.id
  );
