-- Migration 0011: Synchronize delivered status for shipments with recorded date_of_delivery
UPDATE shipments
SET status = 'delivered'
WHERE deleted_at IS NULL
  AND date_of_delivery IS NOT NULL
  AND TRIM(date_of_delivery) != ''
  AND LOWER(COALESCE(status, '')) NOT IN ('delivered', 'cancelled', 'closed');
