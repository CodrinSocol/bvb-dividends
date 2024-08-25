package com.bvb.bvbdividends.Repositories;

import com.bvb.bvbdividends.Entities.Dividend;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.UUID;

public interface DividendRepository extends JpaRepository<Dividend, UUID> {
}
