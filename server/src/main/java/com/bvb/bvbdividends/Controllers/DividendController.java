package com.bvb.bvbdividends.Controllers;

import com.bvb.bvbdividends.DTOs.CompanyDTO;
import com.bvb.bvbdividends.DTOs.DividendDTO;
import com.bvb.bvbdividends.Entities.Company;
import com.bvb.bvbdividends.Entities.Dividend;
import com.bvb.bvbdividends.Repositories.DividendRepository;
import com.bvb.bvbdividends.Services.DividendService;
import org.modelmapper.ModelMapper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.time.LocalDate;
import java.util.List;

@RestController
public class DividendController {

    private DividendService dividendService;
    private Logger log = LoggerFactory.getLogger(DividendController.class);

    @Autowired
    public DividendController(DividendService dividendService) {
        this.dividendService = dividendService;
    }


    @GetMapping("/active-dividends")
    public List<DividendDTO> getActiveDividends() {

        List<Dividend> activeDividends = dividendService.getActiveDividends();

        return activeDividends.stream()
                .map(Dividend::toDividendDTO)
                .toList();
    }

    @GetMapping("/dividends")
    public List<DividendDTO> getAllDividendsPaginated(@RequestParam Integer year,
                                                      @RequestParam Integer pageNumber,
                                                      @RequestParam Integer pageSize
                                                      ) {

        Page<Dividend> dividends = dividendService.getAllDividendsPaginated(year,pageNumber,
            pageSize);

        return dividends.stream()
            .map(Dividend::toDividendDTO)
            .toList();
    }

    @GetMapping("/company-dividends")
    public List<DividendDTO> getDividendsBySymbolPaginated(@RequestParam String symbol, @RequestParam Integer pageNumber, @RequestParam Integer pageSize) {
        Page<Dividend> dividends = dividendService.getAllDividendsBySymbolPaginated(symbol, pageNumber, pageSize);

        return dividends.stream()
            .map(Dividend::toDividendDTO)
            .toList();
    }
}