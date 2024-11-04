package com.bvb.bvbdividends.DTOs;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.UUID;

public class DividendFullDTO {

  private UUID id;
  private String companySymbol;
  private Integer year;
  private BigDecimal dividendForNaturalPerson;
  private BigDecimal dividendForLegalPersons;
  private BigDecimal dividendsTotal;
  private String dividendType;
  private LocalDateTime referenceDateForGMS;
  private LocalDateTime GMSDate;
  private LocalDateTime recordDate;
  private LocalDateTime exDividendDate;
  private LocalDateTime announcementDate;
  private LocalDateTime startPaymentDate;
  private LocalDateTime endPaymentDate;
  private String methodOfDividendDistribution;
}
