package com.bvb.bvbdividends.Scheduling.DividendsForACompany;

import com.bvb.bvbdividends.Entities.Company;
import com.bvb.bvbdividends.Entities.Dividend;
import com.bvb.bvbdividends.wsdl.DividendInfo;
import com.bvb.bvbdividends.wsdl.GetDividends;
import com.bvb.bvbdividends.wsdl.GetDividendsResponse;
import com.bvb.bvbdividends.wsdl.SecurityIdentification;
import java.util.ArrayList;
import java.util.List;
import org.springframework.ws.client.core.support.WebServiceGatewaySupport;
import org.springframework.ws.soap.client.core.SoapActionCallback;

public class DividendsForACompanyClient extends WebServiceGatewaySupport {
  private static final String DIVIDENDS_FOR_A_COMPANY_ENDPOINT =
      "https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx";
  private static final SoapActionCallback DIVIDENDS_FOR_A_COMPANY_CALLBACK =
      new SoapActionCallback("http://www.bvb.ro/GetDividends");
  private static final SecurityIdentification COMPANY_IDENTITY_TYPE = SecurityIdentification.SYMBOL;

  public List<Dividend> getDividendsForACompany(Company company) {
    GetDividends getDividendsForACompanyRequest = new GetDividends();

    getDividendsForACompanyRequest.setIdentityType(COMPANY_IDENTITY_TYPE);
    getDividendsForACompanyRequest.setIdentity(company.getSymbol());

    try {
      GetDividendsResponse getDividendsResponse = (GetDividendsResponse) getWebServiceTemplate()
          .marshalSendAndReceive(DIVIDENDS_FOR_A_COMPANY_ENDPOINT, getDividendsForACompanyRequest,
              DIVIDENDS_FOR_A_COMPANY_CALLBACK);

      return toDividends(company,
          getDividendsResponse.getGetDividendsResult().getDividends().getDividendInfo());
    } catch (Exception e) {
      logger.error("Failed to get dividends for company: " + company.getSymbol());
      logger.error(e.getMessage());
    }

    return new ArrayList<>();
  }

  private List<Dividend> toDividends(Company company, List<DividendInfo> dividendInfoList) {
    return dividendInfoList.stream()
        .map(dividendInfo -> Dividend.fromDividendInfo(company, dividendInfo)).toList();
  }
}
