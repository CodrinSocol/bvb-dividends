package com.bvb.bvbdividends.Scheduling.DividendsForACompany;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.oxm.jaxb.Jaxb2Marshaller;

@Configuration
public class DividendsForACompanyClientConfiguration {

	@Bean
	public Jaxb2Marshaller dividendsForACompanyMarshaller() {
		Jaxb2Marshaller dividendsForACompanyMarshaller = new Jaxb2Marshaller();
		dividendsForACompanyMarshaller.setContextPath("com.bvb.bvbdividends.wsdl");

		return dividendsForACompanyMarshaller;
	}

	@Bean
	public DividendsForACompanyClient dividendsForACompanyClient(Jaxb2Marshaller dividendsForACompanyMarshaller) {
		DividendsForACompanyClient dividendsForACompanyClient = new DividendsForACompanyClient();

		dividendsForACompanyClient.setDefaultUri("https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx/GetLastDividends");
		dividendsForACompanyClient.setMarshaller(dividendsForACompanyMarshaller);
		dividendsForACompanyClient.setUnmarshaller(dividendsForACompanyMarshaller);

		return dividendsForACompanyClient;
	}
}
