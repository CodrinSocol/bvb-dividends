package com.bvb.bvbdividends.Repositories;

import com.bvb.bvbdividends.Entities.Dividend;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.UUID;

@Repository
public interface DividendRepository extends JpaRepository<Dividend, UUID> {

	void deleteAllByCompanySymbol(String companySymbol);

	// get all active dividends. an active dividend is a dividend whose ex-date has not been
	// reached yet.
	List<Dividend> findAllByExDividendDateAfter(LocalDateTime exDividendDate);

	Page<Dividend> findAllByCompanySymbolAndExDividendDateAfterAndExDividendDateBefore(String companySymbol, LocalDateTime startDate, LocalDateTime endDate, Pageable pageable);

	Page<Dividend> findAllByYear(Integer year, Pageable pageable);

	Page<Dividend> findAllByCompanySymbol(String companySymbol, Pageable pageable);
}
