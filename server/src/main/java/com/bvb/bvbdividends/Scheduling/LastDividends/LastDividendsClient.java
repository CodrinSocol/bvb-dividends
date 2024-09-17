package com.bvb.bvbdividends.Scheduling.LastDividends;

import com.bvb.bvbdividends.Entities.Company;
import com.bvb.bvbdividends.wsdl.DividendIdentification;
import com.bvb.bvbdividends.wsdl.GetLastDividends;
import com.bvb.bvbdividends.wsdl.GetLastDividendsResponse;
import org.springframework.ws.client.core.support.WebServiceGatewaySupport;
import org.springframework.ws.soap.client.core.SoapActionCallback;

import java.util.List;
public class LastDividendsClient extends WebServiceGatewaySupport {
	private static final String LAST_DIVIDENDS_ENDPOINT = "https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx";
	private static final SoapActionCallback LAST_DIVIDENDS_CALLBACK = new SoapActionCallback("http://www.bvb.ro/GetLastDividends");

	public List<Company> getLastDividends(Integer nrOfDays) {
		GetLastDividends getLastDividendsRequest = new GetLastDividends();
		getLastDividendsRequest.setNoDays(nrOfDays);

		GetLastDividendsResponse getLastDividendsResponse = (GetLastDividendsResponse) getWebServiceTemplate()
				.marshalSendAndReceive(LAST_DIVIDENDS_ENDPOINT, getLastDividendsRequest, LAST_DIVIDENDS_CALLBACK);

		return toCompanies(getLastDividendsResponse.getGetLastDividendsResult().getDividendIdentification());
	}

	private List<Company> toCompanies(List<DividendIdentification> dividendIdentificationList) {
		return dividendIdentificationList.stream().map(dividendIdentification -> {
			Company company = new Company();
			company.setSymbol(dividendIdentification.getSymbol());
			company.setName(dividendIdentification.getCompany().getCompanyName());

			return company;
		}).distinct().toList();
	}
}
