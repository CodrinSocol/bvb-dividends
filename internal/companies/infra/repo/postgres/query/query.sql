-- name: GetCompany :one
SELECT * FROM company.companies
WHERE symbol = $1;

-- name: CompanyExists :one
SELECT EXISTS (SELECT 1 FROM company.companies WHERE symbol = $1);

-- name: CountCompanies :one
SELECT count(*) FROM company.companies;

-- name: ListCompaniesBySymbols :many
-- The order is restored explicitly: `= ANY` returns rows in whatever order the
-- plan produces, while the symbols were selected in the order the caller asked
-- for.
SELECT * FROM company.companies
WHERE symbol = ANY(@symbols::varchar(20)[])
ORDER BY array_position(@symbols::varchar(20)[], symbol);

-- name: UpsertCompany :batchone
-- The update time only moves when something actually changed, so it means
-- "last changed" rather than "last seen by the importer", which is what
-- AIP-142's update_time is defined to mean. A company whose name BVB has not
-- changed is therefore not returned, and does not count as written.
--
-- The Java service inserted only when absent, so a company that changed its
-- legal name kept the old one forever.
INSERT INTO company.companies (symbol, display_name)
VALUES ($1, $2)
ON CONFLICT (symbol) DO UPDATE
    SET display_name = EXCLUDED.display_name,
        update_time  = now()
WHERE company.companies.display_name IS DISTINCT FROM EXCLUDED.display_name
RETURNING *;
