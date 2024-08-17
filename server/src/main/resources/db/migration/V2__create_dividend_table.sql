CREATE TABLE dividend (
    id UUID PRIMARY KEY,
    company_symbol VARCHAR(20) NOT NULL,
    year INT NOT NULL,
    dividend_for_natural_person DECIMAL(20, 4),
    dividend_for_legal_persons DECIMAL(20, 4),
    dividends_total DECIMAL(20, 4),
    dividend_type VARCHAR(20),
    reference_date_for_gms TIMESTAMP,
    gms_date TIMESTAMP,
    record_date TIMESTAMP,
    ex_dividend_date TIMESTAMP,
    announcement_date TIMESTAMP,
    start_payment_date TIMESTAMP,
    end_payment_date TIMESTAMP,
    method_of_dividend_distribution VARCHAR(255)
);

ALTER TABLE dividend ADD CONSTRAINT fk_dividend_company_symbol FOREIGN KEY (company_symbol) REFERENCES company(symbol);
