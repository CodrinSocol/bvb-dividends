package com.bvb.bvbdividends.Services;

import com.bvb.bvbdividends.DTOs.CompanyDTO;
import com.bvb.bvbdividends.Entities.Company;
import com.bvb.bvbdividends.Repositories.CompanyRepository;
import java.util.Optional;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Pageable;
import org.springframework.data.domain.Sort;
import org.springframework.stereotype.Service;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;

@Service
public class CompanyService {

	private CompanyRepository companyRepository;

	@Autowired
	public CompanyService(CompanyRepository companyRepository) {
		this.companyRepository = companyRepository;
	}

	public Company getCompanyInfo(String symbol) {
		Optional<Company> company = companyRepository.findById(symbol);
		return company.orElse(null);
	}

	public Page<Company> listBVBCompaniesPaginated(Integer pageNumber, Integer pageSize) {
		Pageable pageable = PageRequest.of(pageNumber, pageSize);
		return companyRepository.findAll(pageable);
	}
}
