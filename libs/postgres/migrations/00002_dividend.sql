-- +goose Up
-- Dividends announced by listed companies.
--
-- Every schedule column is a date rather than a timestamp: an ex-dividend date
-- falls on a trading day, not at an instant. BVB sends them as xsd:dateTime
-- with a meaningless time component.
--
-- Every schedule and amount column is nullable, because BVB publishes them
-- progressively as a dividend moves from announcement through approval to
-- payment. Amounts are numeric so they stay exact.
CREATE TABLE dividend (
    -- Derived from the natural key below; see libs/domain.NewID.
    id                             uuid        PRIMARY KEY,
    company_symbol                 varchar(20) NOT NULL REFERENCES company (symbol) ON DELETE CASCADE,
    year                           integer     NOT NULL,
    dividend_type                  text        NOT NULL DEFAULT '',

    gross_per_share_natural_person numeric(20, 4),
    gross_per_share_legal_person   numeric(20, 4),
    total_amount                   numeric(20, 4),

    announcement_date              date,
    gms_reference_date             date,
    gms_date                       date,
    record_date                    date,
    ex_dividend_date               date,
    payment_start_date             date,
    payment_end_date               date,

    distribution_method            text        NOT NULL DEFAULT '',
    created_at                     timestamptz NOT NULL DEFAULT now(),
    updated_at                     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE dividend;
