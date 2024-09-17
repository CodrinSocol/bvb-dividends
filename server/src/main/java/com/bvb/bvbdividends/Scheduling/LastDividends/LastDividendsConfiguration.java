package com.bvb.bvbdividends.Scheduling.LastDividends;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.oxm.jaxb.Jaxb2Marshaller;

@Configuration
public class LastDividendsConfiguration {

	@Bean
	public Jaxb2Marshaller marshaller() {
		Jaxb2Marshaller marshaller = new Jaxb2Marshaller();
		marshaller.setContextPath("com.bvb.bvbdividends.wsdl");

		return marshaller;
	}

	@Bean
	public LastDividendsClient lastDividendsClient(Jaxb2Marshaller marshaller) {
		LastDividendsClient lastDividendsClient = new LastDividendsClient();

		lastDividendsClient.setDefaultUri("https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx/GetLastDividends");
		lastDividendsClient.setMarshaller(marshaller);
		lastDividendsClient.setUnmarshaller(marshaller);

		return lastDividendsClient;
	}
}
