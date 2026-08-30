-- Companies listed on BVB that have announced at least one dividend.
--
-- The slice owns a schema of its own, so its migrations are numbered
-- independently of every other slice's and adding one here never renumbers
-- theirs.
CREATE SCHEMA IF NOT EXISTS company;

-- The ticker symbol is the natural primary key: it is what BVB reports, what
-- the API uses as a resource name segment, and what a dividend references.
--
-- The columns are named after the fields of the API resource, because the
-- filterable fields are derived from the proto message and reach SQL as the
-- column of the same name.
CREATE TABLE company.companies (
    symbol       varchar(20) PRIMARY KEY,
    display_name text        NOT NULL,
    create_time  timestamptz NOT NULL DEFAULT now(),
    update_time  timestamptz NOT NULL DEFAULT now()
);

-- What a list request filters and orders over.
--
-- It exists so that every field the proto declares as filterable has a column,
-- including the ones that are not stored: a resource name is derived from the
-- symbol rather than written down twice.
CREATE VIEW company.companies_filterable AS
SELECT symbol,
       'companies/' || symbol AS name,
       display_name,
       create_time,
       update_time
FROM company.companies;
