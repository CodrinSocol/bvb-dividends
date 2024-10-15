package com.bvb.bvbdividends.Repositories;

import com.bvb.bvbdividends.Entities.Dividend;
import java.time.LocalDateTime;
import java.util.List;
import java.util.UUID;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface DividendRepository extends JpaRepository<Dividend, UUID> {

	void deleteAllByCompanySymbol(String companySymbol);

	// get all active dividends. an active dividend is a dividend whose ex-date has not been
	// reached yet.
	List<Dividend> findAllByExDividendDateAfter(LocalDateTime exDividendDate);

	Page<Dividend> findAllByExDividendDateAfterAndExDividendDateBefore(LocalDateTime startDate, LocalDateTime endDate, Pageable pageable);

	Page<Dividend> findAllByCompanySymbolAndExDividendDateAfterAndExDividendDateBefore(String companySymbol, LocalDateTime startDate, LocalDateTime endDate, Pageable pageable);

}
