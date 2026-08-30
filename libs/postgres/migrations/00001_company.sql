-- +goose Up
-- Companies listed on BVB that have announced at least one dividend.
--
-- The ticker symbol is the natural primary key: it is what BVB reports, what
-- the API uses as a resource name segment, and what dividends reference.
CREATE TABLE company (
    symbol     varchar(20) PRIMARY KEY,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE company;
