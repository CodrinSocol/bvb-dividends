package com.bvb.bvbdividends.Entities;

import com.bvb.bvbdividends.DTOs.DividendDTO;
import com.bvb.bvbdividends.wsdl.DividendInfo;
import jakarta.persistence.*;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.UUID;
import javax.xml.datatype.XMLGregorianCalendar;
import lombok.Getter;
import lombok.Setter;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.SourceType;
import org.hibernate.annotations.UpdateTimestamp;
import org.modelmapper.ModelMapper;

@Entity
@Getter
@Setter
@Table(name = "dividend")
public class Dividend {

	@Id
	@GeneratedValue(strategy = GenerationType.UUID)
	private UUID id;

	@Column(name = "company_symbol", length = 20, nullable = false)
	private String companySymbol;

	@Column(name = "year", nullable = false)
	private Integer year;

	@Column(name = "dividend_for_natural_person")
	private BigDecimal dividendForNaturalPerson;

	@Column(name = "dividend_for_legal_persons")
	private BigDecimal dividendForLegalPersons;

	@Column(name = "dividends_total")
	private BigDecimal dividendsTotal;

	@Column(name = "dividend_type", length = 20)
	private String dividendType;

	@Column(name = "reference_date_for_gms")
	private LocalDateTime referenceDateForGMS;

	@Column(name = "gms_date")
	private LocalDateTime GMSDate;

	@Column(name = "record_date")
	private LocalDateTime recordDate;

	@Column(name = "ex_dividend_date")
	private LocalDateTime exDividendDate;

	@Column(name = "announcement_date")
	private LocalDateTime announcementDate;

	@Column(name = "start_payment_date")
	private LocalDateTime startPaymentDate;

	@Column(name = "end_payment_date")
	private LocalDateTime endPaymentDate;

	@Column(name = "method_of_dividend_distribution")
	private String methodOfDividendDistribution;

	@Column(name = "created_at")
	@CreationTimestamp(source = SourceType.DB)
	private LocalDateTime createdAt;

	@Column(name = "updated_at")
	@UpdateTimestamp(source = SourceType.DB)
	private LocalDateTime updatedAt;

	public static Dividend fromDividendInfo(Company company, DividendInfo dividendInfo) {
		Dividend dividend = new Dividend();

		dividend.setCompanySymbol(company.getSymbol());
		dividend.setYear(dividendInfo.getYear());
		dividend.setDividendForNaturalPerson(dividendInfo.getDividendForNaturalPersons());
		dividend.setDividendForLegalPersons(dividendInfo.getDividendForLegalPersons());
		dividend.setDividendsTotal(dividendInfo.getDividendsTotal());
		dividend.setDividendType(dividendInfo.getDividendType());
		dividend.setReferenceDateForGMS(dividendInfo.getReferenceDateForGMS() != null ? fromXMLGregorianCalendar(dividendInfo.getReferenceDateForGMS()) : null);
		dividend.setGMSDate(dividendInfo.getGMSDate() != null ? fromXMLGregorianCalendar(dividendInfo.getGMSDate()) : null);
		dividend.setRecordDate(dividendInfo.getRecordDate() != null ? fromXMLGregorianCalendar(dividendInfo.getRecordDate()) : null);
		dividend.setExDividendDate(dividendInfo.getExDividendDate() != null ? fromXMLGregorianCalendar(dividendInfo.getExDividendDate()) : null);
		dividend.setAnnouncementDate(dividendInfo.getAnnouncementDate() != null ? fromXMLGregorianCalendar(dividendInfo.getAnnouncementDate()) : null);
		dividend.setStartPaymentDate(dividendInfo.getStartPaymentDate() != null ? fromXMLGregorianCalendar(dividendInfo.getStartPaymentDate()) : null);
		dividend.setEndPaymentDate(dividendInfo.getEndPaymentDate() != null ? fromXMLGregorianCalendar(dividendInfo.getEndPaymentDate()) : null);
		dividend.setMethodOfDividendDistribution(dividendInfo.getMethodOfDividendDistribution());

		return dividend;
	}

	public DividendDTO toDividendDTO() {
		ModelMapper modelMapper = new ModelMapper();
		return modelMapper.map(this, DividendDTO.class);
	}

	//some change fr commit
	private static LocalDateTime fromXMLGregorianCalendar(XMLGregorianCalendar xmlGregorianCalendar) {
		return xmlGregorianCalendar.toGregorianCalendar().toZonedDateTime().toLocalDateTime();
	}
}
