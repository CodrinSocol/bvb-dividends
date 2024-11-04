package com.bvb.bvbdividends.Scheduling;


import com.bvb.bvbdividends.Entities.Company;
import com.bvb.bvbdividends.Entities.Dividend;
import com.bvb.bvbdividends.Repositories.CompanyRepository;
import com.bvb.bvbdividends.Repositories.DividendRepository;
import com.bvb.bvbdividends.Scheduling.DividendsForACompany.DividendsForACompanyClient;
import com.bvb.bvbdividends.Scheduling.LastDividends.LastDividendsClient;
import jakarta.transaction.Transactional;
import java.util.List;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
public class BvbDailyImportTask {
  private static final Logger log = LoggerFactory.getLogger(BvbDailyImportTask.class);
  private static final Integer INITIAL_NR_OF_DAYS_TO_IMPORT = 20 * 365;
  private static final Integer NR_OF_DAYS_TO_IMPORT = 1;


  private final LastDividendsClient lastDividendsClient;
  private final CompanyRepository companyRepository;

  private final DividendsForACompanyClient dividendsForACompanyClient;
  private final DividendRepository dividendRepository;

  public BvbDailyImportTask(LastDividendsClient lastDividendsClient,
                            CompanyRepository companyRepository,
                            DividendsForACompanyClient dividendsForACompanyClient,
                            DividendRepository dividendRepository) {
    this.lastDividendsClient = lastDividendsClient;
    this.companyRepository = companyRepository;
    this.dividendsForACompanyClient = dividendsForACompanyClient;
    this.dividendRepository = dividendRepository;
  }

  /**
   * Import dividends by calling the BVB API.
   * This method is scheduled to run every day at 12:00, Bucharest, Romania Time Zone.
   */
  @Scheduled(cron = "0 0 12 * * *", zone = "Europe/Bucharest")
  @Transactional
  public void importBvbDividends() {
    log.info("Starting to get last BVB companies that announced dividends and their dividends");

    lastDividendsClient.getLastDividends(
            companyRepository.count() == 0
                ? INITIAL_NR_OF_DAYS_TO_IMPORT
                : NR_OF_DAYS_TO_IMPORT)
        .forEach(this::persistCompanyAndDividends);

    log.info("Finished to get last BVB companies that announced dividends and their dividends");
  }

  private void persistCompanyAndDividends(Company company) {
    try {
      List<Dividend> dividends = dividendsForACompanyClient.getDividendsForACompany(company);
      log.info("Persisting company and dividends: {}", company.getSymbol());
      companyRepository.insertIfNotExists(company);

      dividendRepository.deleteAllByCompanySymbol(company.getSymbol());
      dividendRepository.saveAll(dividends);
    } catch (Exception e) {
      log.error("Failed to persist company and dividends: {}", company.getSymbol());
      log.error(e.getMessage());
    }
  }
}
