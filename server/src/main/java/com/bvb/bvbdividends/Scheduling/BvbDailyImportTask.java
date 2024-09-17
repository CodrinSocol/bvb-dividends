package com.bvb.bvbdividends.Scheduling;


import com.bvb.bvbdividends.Repositories.CompanyRepository;
import com.bvb.bvbdividends.Scheduling.LastDividends.LastDividendsClient;
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

	public BvbDailyImportTask(LastDividendsClient lastDividendsClient, CompanyRepository companyRepository) {
		this.lastDividendsClient = lastDividendsClient;
		this.companyRepository = companyRepository;
	}

	public BvbDailyImportTask(LastDividendsClient lastDividendsClient, CompanyRepository companyRepository) {
		this.lastDividendsClient = lastDividendsClient;
		this.companyRepository = companyRepository;
	}

    /**
     * Import dividends by calling the BVB API.
     * This method is scheduled to run every day at 12:00, Bucharest, Romania Time Zone.
     */
//    @Scheduled(cron="0 0 12 * * *", zone="Europe/Bucharest")
	@Scheduled(fixedRate = 30000)
    public void importBvbDividends() {
        log.info("Starting to get last BVB companies that announced dividends");

		lastDividendsClient.getLastDividends(
				companyRepository.count() == 0
						? INITIAL_NR_OF_DAYS_TO_IMPORT
						: NR_OF_DAYS_TO_IMPORT)
				.forEach(companyRepository::insertIfNotExists);

		log.info("Finished to get last BVB dividends");
    }
}
