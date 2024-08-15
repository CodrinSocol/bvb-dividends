package com.bvb.bvbdividends.Controllers;


import com.bvb.bvbdividends.DTOs.CompanyDTO;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;

@RestController
public class CompanyController {

    @GetMapping("/companies")
    public List<String> listBVBCompanies() {
        List<String> companies = List.of("BVB", "BVB2", "BVB3");
        return companies;
    }

    @GetMapping("/company")
    public CompanyDTO getCompanyFromSymbol(@RequestParam String symbol) {
        return new CompanyDTO(symbol, "BVB");
    }
}
