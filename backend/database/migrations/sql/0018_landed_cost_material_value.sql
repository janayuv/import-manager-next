-- Migration 0018: Fix Landed Cost calculation to use Material Value instead of Assessable Value
-- Assessable Value includes freight, insurance, and ex-works charges for customs duty calculation.
-- Adding Assessable Value + Expenses resulted in double-counting freight and ex-works charges.
-- True Landed Cost = Material Value (INR) + BCD + SWS + BOE Interest + Proportional Basic Expenses.

DROP VIEW IF EXISTS report_view;

CREATE VIEW report_view AS
WITH
boe_calc_items AS (
    SELECT
        bc.id AS boe_calc_id,
        bc.shipment_id,
        bc.supplier_name,
        bc.invoice_number,
        COALESCE(
            NULLIF(json_extract(item.value, '$.partNumber'), ''),
            NULLIF(json_extract(item.value, '$.partNo'), '')
        ) AS part_no,
        json_extract(item.value, '$.description') AS boe_description,
        CAST(COALESCE(json_extract(item.value, '$.itemValueInr'), 0.0) AS REAL) AS boe_item_value_inr,
        CAST(COALESCE(json_extract(item.value, '$.assessableValue'), 0.0) AS REAL) AS boe_assessable_value,
        CAST(COALESCE(json_extract(item.value, '$.bcdValue'), 0.0) AS REAL) AS boe_bcd_amount,
        CAST(COALESCE(json_extract(item.value, '$.swsValue'), 0.0) AS REAL) AS boe_sws_amount,
        CAST(COALESCE(json_extract(item.value, '$.igstValue'), 0.0) AS REAL) AS boe_igst_amount,
        CAST(COALESCE(
            json_extract(bc.calculation_result_json, '$.exchangeRate'),
            json_extract(bc.form_values_json, '$.exchangeRate'),
            0.0
        ) AS REAL) AS boe_exchange_rate
    FROM boe_calculations bc
    JOIN json_each(
        COALESCE(
            json_extract(bc.calculation_result_json, '$.items'),
            json_extract(bc.calculation_result_json, '$.calculatedItems'),
            '[]'
        )
    ) AS item
    WHERE bc.deleted_at IS NULL
      AND bc.id = (
          SELECT b2.id
          FROM boe_calculations b2
          WHERE b2.shipment_id = bc.shipment_id AND b2.deleted_at IS NULL
          ORDER BY b2.created_at DESC, b2.id DESC
          LIMIT 1
      )
),
shipment_expenses AS (
    SELECT
        ei.shipment_id,
        SUM(
            CASE
                WHEN COALESCE(e.amount_paise, 0) > 0 THEN e.amount_paise / 100.0
                ELSE COALESCE(e.amount, 0.0)
            END
        ) AS shipment_expenses_basic,
        SUM(
            CASE
                WHEN COALESCE(e.total_amount_paise, 0) > 0 THEN e.total_amount_paise / 100.0
                ELSE COALESCE(e.amount, 0.0)
            END
        ) AS shipment_expenses_total
    FROM expense_invoices ei
    JOIN expenses e ON e.expense_invoice_id = ei.id AND e.deleted_at IS NULL
    WHERE ei.deleted_at IS NULL
    GROUP BY ei.shipment_id
),
boe_assessable AS (
    SELECT
        shipment_id,
        SUM(boe_assessable_value) AS shipment_boe_assessable_total
    FROM boe_calc_items
    GROUP BY shipment_id
),
shipment_boe_interest AS (
    SELECT
        bc.shipment_id,
        SUM(
            CAST(
                COALESCE(
                    json_extract(bc.calculation_result_json, '$.interest'),
                    json_extract(bc.form_values_json, '$.interest'),
                    0.0
                ) AS REAL
            )
        ) AS boe_interest_total
    FROM boe_calculations bc
    WHERE bc.deleted_at IS NULL
      AND bc.id = (
          SELECT b2.id
          FROM boe_calculations b2
          WHERE b2.shipment_id = bc.shipment_id AND b2.deleted_at IS NULL
          ORDER BY b2.created_at DESC, b2.id DESC
          LIMIT 1
      )
    GROUP BY bc.shipment_id
)
SELECT
    sup.supplier_name AS supplier,
    s.supplier_id AS supplier_id,
    s.invoice_number AS invoice_no,
    s.invoice_date AS invoice_date,
    bi.part_no AS part_no,
    COALESCE(NULLIF(i.item_description, ''), bi.boe_description, '') AS description,
    COALESCE(i.unit, '') AS unit,
    COALESCE(ili.quantity, 0.0) AS qty,
    COALESCE(ili.unit_price, 0.0) AS unit_price,
    ROUND(
        COALESCE(
            NULLIF(bi.boe_item_value_inr, 0.0),
            CASE
                WHEN bi.boe_exchange_rate > 0 AND COALESCE(ili.quantity, 0.0) > 0 AND COALESCE(ili.unit_price, 0.0) > 0
                THEN ROUND(ili.quantity * ili.unit_price * bi.boe_exchange_rate, 2)
                ELSE bi.boe_assessable_value
            END
        ),
        2
    ) AS material_value,
    bi.boe_assessable_value AS assessable_value,
    bi.boe_bcd_amount AS bcd_amount,
    bi.boe_sws_amount AS sws_amount,
    bi.boe_igst_amount AS igst_amount,
    ROUND(
        COALESCE(sbi.boe_interest_total, 0.0) *
        (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0.0)),
        2
    ) AS boe_interest,
    ROUND(
        COALESCE(se.shipment_expenses_basic, 0.0) *
        (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0.0)),
        2
    ) AS expenses_total,
    ROUND(
        (
            ROUND(
                COALESCE(
                    NULLIF(bi.boe_item_value_inr, 0.0),
                    CASE
                        WHEN bi.boe_exchange_rate > 0 AND COALESCE(ili.quantity, 0.0) > 0 AND COALESCE(ili.unit_price, 0.0) > 0
                        THEN ROUND(ili.quantity * ili.unit_price * bi.boe_exchange_rate, 2)
                        ELSE bi.boe_assessable_value
                    END
                ),
                2
            ) +
            bi.boe_bcd_amount +
            bi.boe_sws_amount +
            (COALESCE(sbi.boe_interest_total, 0.0) *
              (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0.0))) +
            (COALESCE(se.shipment_expenses_basic, 0.0) *
              (bi.boe_assessable_value / NULLIF(ba.shipment_boe_assessable_total, 0.0)))
        ) / NULLIF(ili.quantity, 0.0),
        4
    ) AS ldc_per_qty
FROM boe_calc_items bi
JOIN shipments s ON s.id = bi.shipment_id AND s.deleted_at IS NULL
JOIN suppliers sup ON sup.id = s.supplier_id AND sup.deleted_at IS NULL
JOIN invoices inv ON inv.shipment_id = s.id AND inv.deleted_at IS NULL
JOIN items i ON UPPER(i.part_number) = UPPER(bi.part_no) AND i.deleted_at IS NULL
JOIN invoice_line_items ili ON ili.invoice_id = inv.id AND ili.item_id = i.id AND ili.deleted_at IS NULL
LEFT JOIN shipment_expenses se ON se.shipment_id = s.id
LEFT JOIN boe_assessable ba ON ba.shipment_id = s.id
LEFT JOIN shipment_boe_interest sbi ON sbi.shipment_id = s.id;
