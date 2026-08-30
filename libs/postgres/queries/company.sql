-- name: GetCompany :one
SELECT symbol, name, created_at, updated_at
FROM company
WHERE symbol = $1;

-- name: CompanyExists :one
SELECT EXISTS (SELECT 1 FROM company WHERE symbol = $1);

-- name: CountCompanies :one
SELECT count(*) FROM company;

-- name: UpsertCompany :exec
-- The update time only moves when something actually changed, so it means
-- "last changed" rather than "last seen by the importer", which is what
-- AIP-142's update_time is defined to mean.
INSERT INTO company (symbol, name)
VALUES ($1, $2)
ON CONFLICT (symbol) DO UPDATE
    SET name       = EXCLUDED.name,
        updated_at = now()
WHERE company.name IS DISTINCT FROM EXCLUDED.name;
