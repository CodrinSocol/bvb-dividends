-- +goose Up
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
CREATE UNIQUE INDEX dividend_natural_key_idx
    ON dividend (company_symbol, year, dividend_type, ex_dividend_date)
    NULLS NOT DISTINCT;

-- Listing one company's dividends newest first is the most common query, and
-- the API's default ordering.
CREATE INDEX dividend_company_ex_date_idx
    ON dividend (company_symbol, ex_dividend_date DESC);

-- Listing across all companies, which is what the AIP-159 `companies/-`
-- wildcard resolves to.
CREATE INDEX dividend_ex_date_idx
    ON dividend (ex_dividend_date DESC);

-- +goose Down
DROP INDEX dividend_ex_date_idx;
DROP INDEX dividend_company_ex_date_idx;
DROP INDEX dividend_natural_key_idx;
