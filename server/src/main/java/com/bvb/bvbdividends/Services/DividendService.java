package com.bvb.bvbdividends.Services;

import com.bvb.bvbdividends.Entities.Dividend;
import com.bvb.bvbdividends.Repositories.DividendRepository;
import java.time.LocalDateTime;
import java.util.List;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Pageable;
import org.springframework.stereotype.Service;

@Service
public class DividendService {

	private final DividendRepository dividendRepository;

	@Autowired
	public DividendService(DividendRepository dividendRepository) {
		this.dividendRepository = dividendRepository;
	}

	public List<Dividend> getActiveDividends() {
		LocalDateTime now = LocalDateTime.now();
		return dividendRepository.findAllByExDividendDateAfter(now);
	}

	public Page<Dividend> getAllDividendsPaginated(Integer year, Integer pageNumber,
												   Integer pageSize) {

		Pageable pageable = PageRequest.of(pageNumber, pageSize);
		return dividendRepository.findAllByYear(year, pageable);
	}

	public Page<Dividend> getAllDividendsBySymbolPaginated(String symbol, Integer pageNumber,
															Integer pageSize) {
		Pageable pageable = PageRequest.of(pageNumber, pageSize);
		return dividendRepository.findAllByCompanySymbol(symbol, pageable);
	}

}
