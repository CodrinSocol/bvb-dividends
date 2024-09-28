package com.bvb.bvbdividends.Repositories;

import com.bvb.bvbdividends.Entities.Dividend;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.UUID;

@Repository
public interface DividendRepository extends JpaRepository<Dividend, UUID> {

	void deleteAllByCompanySymbol(String companySymbol);
}
