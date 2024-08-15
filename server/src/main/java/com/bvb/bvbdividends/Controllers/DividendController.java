package com.bvb.bvbdividends.Controllers;

import com.bvb.bvbdividends.DTOs.DividendDTO;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.util.Date;
import java.util.List;

@RestController
public class DividendController {

    @GetMapping("/active-dividends")
    public List<DividendDTO> getActiveDividendsPaginated(@RequestParam int pageNumber, @RequestParam int pageSize) {
        return List.of(
                new DividendDTO("1", "BVB", 2020),
                new DividendDTO("2", "BVB", 2021),
                new DividendDTO("3", "BVB", 2022)
        );
    }

    @GetMapping("/dividends")
    public List<DividendDTO> getAllDividendsPaginated(@RequestParam int pageNumber, @RequestParam int pageSize) {
        return List.of(
                new DividendDTO("1", "BVB", 2020),
                new DividendDTO("2", "BVB", 2021),
                new DividendDTO("3", "BVB", 2022)
        );
    }

    @GetMapping("/dividends-by-symbol")
    public List<DividendDTO> getDividendsBySymbolPaginated(@RequestParam String symbol, @RequestParam int pageNumber, @RequestParam int pageSize) {
        return List.of(
                new DividendDTO("1", symbol, 2020),
                new DividendDTO("2", symbol, 2021),
                new DividendDTO("3", symbol, 2022)
        );
    }

    @GetMapping("calendar-dividends")
    public List<DividendDTO> getCalendarDividendsPaginated(@RequestParam Date startDate,@RequestParam Date endDate) {
        return List.of(
                new DividendDTO("1", "BVB", 2020),
                new DividendDTO("2", "BVB", 2021),
                new DividendDTO("3", "BVB", 2022)
        );
    }
}
