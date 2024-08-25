package com.bvb.bvbdividends.Scheduling;


import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.text.SimpleDateFormat;

@Component
public class BvbDailyImportTask {

    private static final Logger log = LoggerFactory.getLogger(BvbDailyImportTask.class);

    private static final SimpleDateFormat dateFormat = new SimpleDateFormat("HH:mm:ss");


    /**
     * Import dividends by calling the BVB API.
     * This method is scheduled to run every day at 12:00, Bucharest, Romania Time Zone.
     */
    @Scheduled(cron="0 0 12 * * *", zone="Europe/Bucharest")
    public void importBvbDividends() {
        log.info("The time is now {}", dateFormat.format(System.currentTimeMillis()));
    }
}
