-- name: GetDividend :one
SELECT * FROM dividend.dividends_filterable
WHERE id = $1;

-- name: ListDividendsByIDs :many
-- The order is restored explicitly: `= ANY` returns rows in whatever order the
-- plan produces, while the identifiers were selected in the order the caller
-- asked for.
SELECT * FROM dividend.dividends_filterable
WHERE id = ANY(@ids::uuid[])
ORDER BY array_position(@ids::uuid[], id);

-- name: UpsertDividend :batchone
-- Conflicts resolve on the identifier, which is derived from the dividend's
-- natural key, so a re-import updates the row it created the first time rather
-- than inserting a second copy. The creation time and the resource name
-- therefore survive for as long as BVB keeps reporting the dividend.
--
-- The previous service deleted every one of a company's dividends and
-- re-inserted them with fresh random identifiers on each daily run, so every
-- saved link broke within a day.
--
-- Nothing is written when BVB reports a dividend it has not changed, so the
-- update time keeps meaning "last changed" rather than "last seen".
INSERT INTO dividend.dividends (id, company_symbol, year, dividend_type,
                                gross_per_share_natural_person, gross_per_share_legal_person, total_amount,
                                schedule_announcement_date, schedule_gms_reference_date, schedule_gms_date,
                                schedule_record_date, schedule_ex_dividend_date, schedule_payment_start_date,
                                schedule_payment_end_date, distribution_method)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (id) DO UPDATE
    SET company_symbol                 = EXCLUDED.company_symbol,
        year                           = EXCLUDED.year,
        dividend_type                  = EXCLUDED.dividend_type,
        gross_per_share_natural_person = EXCLUDED.gross_per_share_natural_person,
        gross_per_share_legal_person   = EXCLUDED.gross_per_share_legal_person,
        total_amount                   = EXCLUDED.total_amount,
        schedule_announcement_date     = EXCLUDED.schedule_announcement_date,
        schedule_gms_reference_date    = EXCLUDED.schedule_gms_reference_date,
        schedule_gms_date              = EXCLUDED.schedule_gms_date,
        schedule_record_date           = EXCLUDED.schedule_record_date,
        schedule_ex_dividend_date      = EXCLUDED.schedule_ex_dividend_date,
        schedule_payment_start_date    = EXCLUDED.schedule_payment_start_date,
        schedule_payment_end_date      = EXCLUDED.schedule_payment_end_date,
        distribution_method            = EXCLUDED.distribution_method,
        update_time                    = now()
WHERE (dividend.dividends.gross_per_share_natural_person, dividend.dividends.gross_per_share_legal_person,
       dividend.dividends.total_amount, dividend.dividends.schedule_announcement_date,
       dividend.dividends.schedule_gms_reference_date, dividend.dividends.schedule_gms_date,
       dividend.dividends.schedule_record_date, dividend.dividends.schedule_payment_start_date,
       dividend.dividends.schedule_payment_end_date, dividend.dividends.distribution_method)
      IS DISTINCT FROM
      (EXCLUDED.gross_per_share_natural_person, EXCLUDED.gross_per_share_legal_person,
       EXCLUDED.total_amount, EXCLUDED.schedule_announcement_date,
       EXCLUDED.schedule_gms_reference_date, EXCLUDED.schedule_gms_date,
       EXCLUDED.schedule_record_date, EXCLUDED.schedule_payment_start_date,
       EXCLUDED.schedule_payment_end_date, EXCLUDED.distribution_method)
RETURNING id;
