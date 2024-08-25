package com.bvb.bvbdividends.Controllers;


import com.bvb.bvbdividends.DTOs.CompanyDTO;
import com.bvb.bvbdividends.DTOs.CompanySymbolDTO;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.Stream;

@RestController
public class CompanyController {

    @GetMapping("/companies")
    public List<CompanySymbolDTO> listBVBCompanies() {
        return Stream.of("BVB", "BVB2", "BVB3").map(CompanySymbolDTO::new).collect(Collectors.toList());
    }

    @GetMapping("/company")
    public CompanyDTO getCompanyFromSymbol(@RequestParam String symbol) {
        return new CompanyDTO(symbol, "BVB");
    }
}
