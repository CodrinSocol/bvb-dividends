package com.bvb.bvbdividends.Controllers;


import com.bvb.bvbdividends.DTOs.CompanyDTO;
import com.bvb.bvbdividends.Entities.Company;
import com.bvb.bvbdividends.Exceptions.CompanyNotFoundException;
import com.bvb.bvbdividends.Services.CompanyService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;
import java.util.stream.Collectors;

@RestController
public class CompanyController {

    private CompanyService companyService;

    @Autowired
    public CompanyController(CompanyService companyService) {
        this.companyService = companyService;
    }

    @GetMapping("/companies")
    public List<CompanyDTO> listBVBCompaniesPaginated(@RequestParam Integer pageNumber,
                                                      @RequestParam Integer pageSize) {
        Page<Company> companies = companyService.listBVBCompaniesPaginated(pageNumber, pageSize);
        return companies.stream().map(Company::toCompanyDTO).collect(Collectors.toList());
    }

    @GetMapping("/company")
    public CompanyDTO getCompanyFromSymbol(@RequestParam String symbol) {
        Company company = companyService.getCompanyInfo(symbol);

        if (company == null) {
            throw new CompanyNotFoundException(symbol);
        }
        return company.toCompanyDTO();
    }
}


