package com.bvb.bvbdividends.Entities;

import jakarta.persistence.*;
import lombok.Getter;
import lombok.Setter;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.SourceType;
import org.hibernate.annotations.UpdateTimestamp;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Objects;

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
	private List<Dividend> dividend;

	@Override
	public boolean equals(Object o) {
		if (this == o) return true;
		if (o == null || getClass() != o.getClass()) return false;
		Company company = (Company) o;
		return Objects.equals(symbol, company.symbol) && Objects.equals(name, company.name);
	}

	@Override
	public int hashCode() {
		return Objects.hash(symbol, name);
	}
}
