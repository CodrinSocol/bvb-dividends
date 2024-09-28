package com.bvb.bvbdividends.Entities;

import com.bvb.bvbdividends.wsdl.DividendIdentification;
import jakarta.persistence.*;
import lombok.Getter;
import lombok.Setter;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.SourceType;
import org.hibernate.annotations.UpdateTimestamp;

import java.time.LocalDateTime;
import java.util.List;

@Entity
@Setter
@Getter
@Table(name = "company")
public class Company {

	@Id
	private String symbol;

	@Column(name = "name", nullable = false)
	private String name;

	@Column(name = "created_at", nullable = false)
	@CreationTimestamp(source = SourceType.DB)
	private LocalDateTime createdAt;

	@Column(name = "updated_at", nullable = false)
	@UpdateTimestamp(source = SourceType.DB)
	private LocalDateTime updatedAt;

	@OneToMany(fetch = FetchType.LAZY)
	@JoinColumn(name = "company_symbol", referencedColumnName = "symbol")
	private List<Dividend> dividends;

	public static Company fromDividendIdentification(DividendIdentification dividendIdentification) {
		Company company = new Company();

		company.setSymbol(dividendIdentification.getSymbol());
		company.setName(dividendIdentification.getCompany().getCompanyName());

		return company;
	}
}
