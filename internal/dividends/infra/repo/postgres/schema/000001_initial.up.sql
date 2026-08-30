-- Dividends announced by listed companies.
--
-- The slice owns a schema of its own; the foreign key into company.companies is
-- why its namespace declares a dependency on that one, so the schemas are
-- migrated in an order in which the reference can be created.
CREATE SCHEMA IF NOT EXISTS dividend;

-- Every schedule column is a date rather than a timestamp: an ex-dividend date
-- falls on a trading day, not at an instant. BVB sends them as xsd:dateTime
-- with a meaningless time component.
--
-- Every schedule and amount column is nullable, because BVB publishes them
-- progressively as a dividend moves from announcement through approval to
-- payment. Amounts are numeric so they stay exact.
--
-- The schedule columns carry the schedule_ prefix because the API nests them
-- under a Schedule message, and a filter names them as schedule.ex_dividend_date;
-- a filterable field reaches SQL as the column of the same name with the dot
-- replaced.
CREATE TABLE dividend.dividends (
    -- Derived from the natural key below; see internal/dividends.NewID.
    id                             uuid        PRIMARY KEY,
    company_symbol                 varchar(20) NOT NULL REFERENCES company.companies (symbol) ON DELETE CASCADE,
    year                           integer     NOT NULL,
    dividend_type                  text        NOT NULL DEFAULT '',

    gross_per_share_natural_person numeric(20, 4),
    gross_per_share_legal_person   numeric(20, 4),
    total_amount                   numeric(20, 4),

    schedule_announcement_date     date,
    schedule_gms_reference_date    date,
    schedule_gms_date              date,
    schedule_record_date           date,
    schedule_ex_dividend_date      date,
    schedule_payment_start_date    date,
    schedule_payment_end_date      date,

    distribution_method            text        NOT NULL DEFAULT '',
    create_time                    timestamptz NOT NULL DEFAULT now(),
    update_time                    timestamptz NOT NULL DEFAULT now()
);

-- Asserts that a dividend's identifier really is a function of its natural key.
--
-- The primary key already enforces this indirectly, since the id is derived
-- from exactly these four columns. Stating it directly means that if any code
-- path ever inserts a dividend with an id it generated some other way, the
-- database rejects it instead of quietly storing a duplicate of a dividend it
-- no longer recognises as the same one.
--
-- NULLS NOT DISTINCT is what makes this work: BVB often has no ex-dividend date
-- for a freshly announced dividend, and under the default NULL handling every
-- such row would count as unique and duplicate on every import.
CREATE UNIQUE INDEX dividends_natural_key_idx
    ON dividend.dividends (company_symbol, year, dividend_type, schedule_ex_dividend_date)
    NULLS NOT DISTINCT;

-- Listing one company's dividends newest first is the most common query, and
-- the API's default ordering.
CREATE INDEX dividends_company_ex_date_idx
    ON dividend.dividends (company_symbol, schedule_ex_dividend_date DESC);

-- Listing across all companies, which is what the AIP-159 `companies/-`
-- wildcard resolves to.
CREATE INDEX dividends_ex_date_idx
    ON dividend.dividends (schedule_ex_dividend_date DESC);

-- What a read, a filter and an ordering all go through.
--
-- It supplies the two fields the API has that the table does not: the resource
-- name, which is derived from the company and the identifier, and the lifecycle
-- state, which is a function of the schedule and of today rather than something
-- stored. Deriving the state here rather than storing it means it can never go
-- stale, and reading it from here rather than recomputing it after the fact
-- means a filter on the state and the state in the response cannot disagree.
CREATE VIEW dividend.dividends_filterable AS
SELECT id,
       company_symbol,
       'companies/' || company_symbol || '/dividends/' || id AS name,
       year,
       dividend_type,
       gross_per_share_natural_person,
       gross_per_share_legal_person,
       total_amount,
       schedule_announcement_date,
       schedule_gms_reference_date,
       schedule_gms_date,
       schedule_record_date,
       schedule_ex_dividend_date,
       schedule_payment_start_date,
       schedule_payment_end_date,
       distribution_method,
       CASE
           -- BVB reported no ex-dividend date, so the stage cannot be derived.
           WHEN schedule_ex_dividend_date IS NULL
               THEN 'STATE_UNDATED'
           -- The ex-dividend date is the first day the share trades without
           -- entitlement, so a dividend stops being claimable on that date,
           -- not after it.
           WHEN schedule_ex_dividend_date > CURRENT_DATE
               THEN 'STATE_ANNOUNCED'
           WHEN schedule_payment_end_date IS NOT NULL AND schedule_payment_end_date < CURRENT_DATE
               THEN 'STATE_PAID'
           WHEN schedule_payment_start_date IS NOT NULL AND schedule_payment_start_date <= CURRENT_DATE
               THEN 'STATE_PAYING'
           ELSE 'STATE_EX_PASSED'
       END AS state,
       create_time,
       update_time
FROM dividend.dividends;
