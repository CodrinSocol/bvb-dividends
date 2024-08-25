package com.bvb.bvbdividends.Repositories;

import com.bvb.bvbdividends.Entities.Company;
import org.springframework.data.jpa.repository.JpaRepository;

public interface CompanyRepository extends JpaRepository<Company, String> {
}
