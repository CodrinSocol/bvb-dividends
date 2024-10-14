package com.bvb.bvbdividends.Services;

import com.bvb.bvbdividends.Entities.Dividend;
import com.bvb.bvbdividends.Repositories.DividendRepository;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
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

	public Page<Dividend> getAllDividendsPaginated(Integer pageNumber,
												   Integer pageSize, String companySymbol,
												   Boolean onlyActive, LocalDate startDate,
												   LocalDate endDate) {



		Pageable pageable = PageRequest.of(pageNumber, pageSize);
		return dividendRepository.findAll(companySymbol,
			pageable);
	}

	public Optional<Dividend> getDividendById(UUID dividendId) {
		return dividendRepository.findById(dividendId);
	}

//	public Page<Dividend> getAllDividendsBySymbolPaginated(String symbol, Integer pageNumber,
//															Integer pageSize) {
//		Pageable pageable = PageRequest.of(pageNumber, pageSize);
//		return dividendRepository.findAllByCompanySymbol(symbol, pageable);
//	}

}
