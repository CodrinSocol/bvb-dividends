package com.bvb.bvbdividends.Exceptions;

import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.ResponseStatus;

@ResponseStatus(HttpStatus.NOT_FOUND)
public class CompanyNotFoundException extends RuntimeException {
	public CompanyNotFoundException(String symbol) {

		super("Company with symbol " + symbol + " not found");
	}
}
