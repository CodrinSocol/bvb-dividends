package com.bvb.bvbdividends.DTOs;

import java.time.LocalDateTime;
import java.util.UUID;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@AllArgsConstructor
@NoArgsConstructor
@Getter
@Setter
public class DividendDTO {
  private UUID id;
  private String companySymbol;
  private Integer year;

  private LocalDateTime exDividendDate;
  private LocalDateTime startPaymentDate;
  private LocalDateTime recordDate;
  private LocalDateTime announcementDate;

  private Double dividendForNaturalPerson;
  private Double dividendForLegalPersons;
}
