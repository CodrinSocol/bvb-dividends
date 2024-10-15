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
												   LocalDate startDate,
												   LocalDate endDate) {

		LocalDateTime startDateTime = startDate != null ? startDate.atStartOfDay() :
			LocalDateTime.parse("1900-01-01T00:00:00");
		LocalDateTime endDateTime = endDate != null ? endDate.atStartOfDay() :
			LocalDateTime.parse("2100-01-01T00:00:00");

		Pageable pageable = PageRequest.of(pageNumber, pageSize);

		if(companySymbol == null) {
			return dividendRepository.findAllByExDividendDateAfterAndExDividendDateBefore(startDateTime, endDateTime, pageable);
		}

		return dividendRepository.findAllByCompanySymbolAndExDividendDateAfterAndExDividendDateBefore(companySymbol, startDateTime, endDateTime, pageable);
	}

	public Optional<Dividend> getDividendById(UUID dividendId) {
		return dividendRepository.findById(dividendId);
	}

}
