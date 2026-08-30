-- name: GetDividend :one
SELECT id, company_symbol, year, dividend_type,
       gross_per_share_natural_person, gross_per_share_legal_person, total_amount,
       announcement_date, gms_reference_date, gms_date, record_date,
       ex_dividend_date, payment_start_date, payment_end_date,
       distribution_method, created_at, updated_at
FROM dividend
WHERE id = $1;

-- name: UpsertDividend :execrows
-- Conflicts resolve on the identifier, which is derived from the dividend's
-- natural key, so a re-import updates the row it created the first time rather
-- than inserting a second copy. The creation time and the resource name
-- therefore survive for as long as BVB keeps reporting the dividend.
INSERT INTO dividend (id, company_symbol, year, dividend_type,
                      gross_per_share_natural_person, gross_per_share_legal_person, total_amount,
                      announcement_date, gms_reference_date, gms_date, record_date,
                      ex_dividend_date, payment_start_date, payment_end_date,
                      distribution_method)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (id) DO UPDATE
    SET company_symbol                 = EXCLUDED.company_symbol,
        year                           = EXCLUDED.year,
        dividend_type                  = EXCLUDED.dividend_type,
        gross_per_share_natural_person = EXCLUDED.gross_per_share_natural_person,
        gross_per_share_legal_person   = EXCLUDED.gross_per_share_legal_person,
        total_amount                   = EXCLUDED.total_amount,
        announcement_date              = EXCLUDED.announcement_date,
        gms_reference_date             = EXCLUDED.gms_reference_date,
        gms_date                       = EXCLUDED.gms_date,
        record_date                    = EXCLUDED.record_date,
        ex_dividend_date               = EXCLUDED.ex_dividend_date,
        payment_start_date             = EXCLUDED.payment_start_date,
        payment_end_date               = EXCLUDED.payment_end_date,
        distribution_method            = EXCLUDED.distribution_method,
        updated_at                     = now()
WHERE (dividend.gross_per_share_natural_person, dividend.gross_per_share_legal_person,
       dividend.total_amount, dividend.announcement_date, dividend.gms_reference_date,
       dividend.gms_date, dividend.record_date, dividend.payment_start_date,
       dividend.payment_end_date, dividend.distribution_method)
      IS DISTINCT FROM
      (EXCLUDED.gross_per_share_natural_person, EXCLUDED.gross_per_share_legal_person,
       EXCLUDED.total_amount, EXCLUDED.announcement_date, EXCLUDED.gms_reference_date,
       EXCLUDED.gms_date, EXCLUDED.record_date, EXCLUDED.payment_start_date,
       EXCLUDED.payment_end_date, EXCLUDED.distribution_method);
