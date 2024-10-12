package com.bvb.bvbdividends.Controllers;

import com.bvb.bvbdividends.DTOs.DividendDTO;
import com.bvb.bvbdividends.Entities.Dividend;
import com.bvb.bvbdividends.Services.DividendService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;

@RestController
public class DividendController {

    private final DividendService dividendService;

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