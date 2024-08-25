package com.bvb.bvbdividends.Entities;

import jakarta.persistence.*;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.SourceType;
import org.hibernate.annotations.UpdateTimestamp;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.UUID;

@Entity
@Table(name = "dividend")
public class Dividend {

	@Id
	@GeneratedValue(strategy = GenerationType.UUID)
	private UUID id;

	@Column(name = "company_symbol", length = 20, nullable = false)
	private String companySymbol;

	@Column(name = "year", nullable = false)
	private Integer year;

	@Column(name = "dividend_for_natural_person")
	private BigDecimal dividendForNaturalPerson;

	@Column(name = "dividend_for_legal_persons")
	private BigDecimal dividendForLegalPersons;

	@Column(name = "dividends_total")
	private BigDecimal dividendsTotal;

	@Column(name = "dividend_type", length = 20)
	private String dividendType;

	@Column(name = "reference_date_for_gms")
	private LocalDateTime referenceDateForGMS;

	@Column(name = "gms_date")
	private LocalDateTime GMSDate;

	@Column(name = "record_date")
	private LocalDateTime recordDate;

	@Column(name = "ex_dividend_date")
	private LocalDateTime exDividendDate;

	@Column(name = "announcement_date")
	private LocalDateTime announcementDate;

	@Column(name = "start_payment_date")
	private LocalDateTime startPaymentDate;

	@Column(name = "end_payment_date")
	private LocalDateTime endPaymentDate;

	@Column(name = "method_of_dividend_distribution")
	private String methodOfDividendDistribution;

	@Column(name = "created_at")
	@CreationTimestamp(source = SourceType.DB)
	private LocalDateTime createdAt;

	@Column(name = "updated_at")
	@UpdateTimestamp(source = SourceType.DB)
	private LocalDateTime updatedAt;

}
