package com.bvb.bvbdividends.Controllers;

import com.bvb.bvbdividends.DTOs.DividendDTO;
import com.bvb.bvbdividends.Entities.Dividend;
import com.bvb.bvbdividends.Services.DividendService;
import java.time.LocalDate;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.format.annotation.DateTimeFormat;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class DividendController {

    private final DividendService dividendService;

    @Autowired
    public DividendController(DividendService dividendService) {
        this.dividendService = dividendService;
    }


//    @GetMapping("/active-dividends")
//    public List<DividendDTO> getActiveDividends() {
//
//        List<Dividend> activeDividends = dividendService.getActiveDividends();
//
//        return activeDividends.stream()
//                .map(Dividend::toDividendDTO)
//                .toList();
//    }

    @GetMapping("/dividends")
    public ResponseEntity<List<DividendDTO>> getDividendsPaginatedAndFiltered(
      @RequestParam Integer pageNumber,
      @RequestParam Integer pageSize,
      @RequestParam(required = false) String companySymbol,
      @RequestParam(required = false) Boolean onlyActive,
      @RequestParam(required = false) @DateTimeFormat(iso = DateTimeFormat.ISO.DATE) LocalDate startDate,
      @RequestParam(required = false) @DateTimeFormat(iso = DateTimeFormat.ISO.DATE) LocalDate endDate) {

        Page<Dividend> dividends = dividendService.getAllDividendsPaginated(pageNumber,
            pageSize, companySymbol, onlyActive, startDate, endDate);



        return ResponseEntity.ok(dividends.stream()
            .map(Dividend::toDividendDTO)
            .toList());
    }

    @GetMapping("/dividend")
    public ResponseEntity<Dividend> getDividendBySymbolAndYear(@RequestParam UUID dividendId) {
        Optional<Dividend> dividend = dividendService.getDividendById(dividendId);

		return dividend.map(ResponseEntity::ok).orElseGet(() -> ResponseEntity.notFound().build());
	}


//    @GetMapping("/company-dividends")
//    public List<DividendDTO> getDividendsBySymbolPaginated(@RequestParam String symbol, @RequestParam Integer pageNumber, @RequestParam Integer pageSize) {
//        Page<Dividend> dividends = dividendService.getAllDividendsBySymbolPaginated(symbol, pageNumber, pageSize);
//
//        return dividends.stream()
//            .map(Dividend::toDividendDTO)
//            .toList();
//    }
//
}