package com.bvb.bvbdividends.Controllers;


import com.bvb.bvbdividends.Services.CompanyService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class CompanyController {

    private final CompanyService companyService;

    @Autowired
    public CompanyController(CompanyService companyService) {
        this.companyService = companyService;
    }

//    @GetMapping("/companies")
//    public List<CompanyDTO> listBVBCompaniesPaginated(@RequestParam Integer pageNumber,
//                                                      @RequestParam Integer pageSize) {
//        Page<Company> companies = companyService.listBVBCompaniesPaginated(pageNumber, pageSize);
//        return companies.stream().map(Company::toCompanyDTO).collect(Collectors.toList());
//    }
//
//    @GetMapping("/company")
//    public CompanyDTO getCompanyFromSymbol(@RequestParam String symbol) {
//        Company company = companyService.getCompanyInfo(symbol);
//
//        if (company == null) {
//            throw new CompanyNotFoundException(symbol);
//        }
//        return company.toCompanyDTO();
//    }
}


