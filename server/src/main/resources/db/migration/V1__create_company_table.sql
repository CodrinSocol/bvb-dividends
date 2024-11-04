CREATE TABLE company
(
    symbol     VARCHAR(20) PRIMARY KEY,
    name       VARCHAR(255)                NOT NULL,
    created_at TIMESTAMP without time zone NOT NULL,
    updated_at TIMESTAMP without time zone NOT NULL
);
