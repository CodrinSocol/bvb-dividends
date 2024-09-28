package com.bvb.bvbdividends.Scheduling.LastDividends;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.oxm.jaxb.Jaxb2Marshaller;

@Configuration
public class LastDividendsConfiguration {

	@Bean
	public Jaxb2Marshaller lastDividendsMarshaller() {
		Jaxb2Marshaller lastDividendsMarshaller = new Jaxb2Marshaller();
		lastDividendsMarshaller.setContextPath("com.bvb.bvbdividends.wsdl");

		return lastDividendsMarshaller;
	}

	@Bean
	public LastDividendsClient lastDividendsClient(Jaxb2Marshaller lastDividendsMarshaller) {
		LastDividendsClient lastDividendsClient = new LastDividendsClient();

		lastDividendsClient.setDefaultUri("https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx/GetLastDividends");
		lastDividendsClient.setMarshaller(lastDividendsMarshaller);
		lastDividendsClient.setUnmarshaller(lastDividendsMarshaller);

		return lastDividendsClient;
	}
}
