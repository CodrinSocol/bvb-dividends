CREATE TABLE dividend
(
    id                              UUID PRIMARY KEY,
    company_symbol                  VARCHAR(20)                 NOT NULL,
    year                            INT                         NOT NULL,
    dividend_for_natural_person     DECIMAL(20, 4),
    dividend_for_legal_persons      DECIMAL(20, 4),
    dividends_total                 DECIMAL(20, 4),
    dividend_type                   VARCHAR(20),
    reference_date_for_gms          TIMESTAMP without time zone,
    gms_date                        TIMESTAMP without time zone,
    record_date                     TIMESTAMP without time zone,
    ex_dividend_date                TIMESTAMP without time zone,
    announcement_date               TIMESTAMP without time zone,
    start_payment_date              TIMESTAMP without time zone,
    end_payment_date                TIMESTAMP without time zone,
    method_of_dividend_distribution VARCHAR(255),
    created_at                      TIMESTAMP without time zone NOT NULL,
    updated_at                      TIMESTAMP without time zone NOT NULL
);

ALTER TABLE dividend
    ADD CONSTRAINT fk_dividend_company_symbol FOREIGN KEY (company_symbol) REFERENCES company (symbol);
