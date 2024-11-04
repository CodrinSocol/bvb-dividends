package com.bvb.bvbdividends.Repositories;

import com.bvb.bvbdividends.Entities.Company;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface CompanyRepository extends JpaRepository<Company, String> {

  default void insertIfNotExists(Company company) {
    if (!existsById(company.getSymbol())) {
      save(company);
    }
  }
}
