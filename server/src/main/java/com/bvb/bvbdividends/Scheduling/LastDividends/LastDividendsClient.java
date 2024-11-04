package com.bvb.bvbdividends.Scheduling.LastDividends;

import com.bvb.bvbdividends.Entities.Company;
import com.bvb.bvbdividends.wsdl.DividendIdentification;
import com.bvb.bvbdividends.wsdl.GetLastDividends;
import com.bvb.bvbdividends.wsdl.GetLastDividendsResponse;
import java.util.ArrayList;
import java.util.List;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ws.client.core.support.WebServiceGatewaySupport;
import org.springframework.ws.soap.client.core.SoapActionCallback;

public class LastDividendsClient extends WebServiceGatewaySupport {
  private static final String LAST_DIVIDENDS_ENDPOINT =
      "https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx";
  private static final SoapActionCallback LAST_DIVIDENDS_CALLBACK =
      new SoapActionCallback("http://www.bvb.ro/GetLastDividends");
  private static final Logger logger = LoggerFactory.getLogger(LastDividendsClient.class);

  public List<Company> getLastDividends(Integer nrOfDays) {
    GetLastDividends getLastDividendsRequest = new GetLastDividends();
    getLastDividendsRequest.setNoDays(nrOfDays);

    try {
      GetLastDividendsResponse getLastDividendsResponse =
          (GetLastDividendsResponse) getWebServiceTemplate()
              .marshalSendAndReceive(LAST_DIVIDENDS_ENDPOINT, getLastDividendsRequest,
                  LAST_DIVIDENDS_CALLBACK);

      return toCompanies(
          getLastDividendsResponse.getGetLastDividendsResult().getDividendIdentification());
    } catch (Exception e) {
      logger.error("Failed to get last dividends");
      logger.error(e.getMessage());
    }
    return new ArrayList<>();
  }

  private List<Company> toCompanies(List<DividendIdentification> dividendIdentificationList) {
    return dividendIdentificationList.stream().map(Company::fromDividendIdentification).distinct()
        .toList();
  }
}
