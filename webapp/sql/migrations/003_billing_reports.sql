-- Apply once before deploying the new app. No history rewrite.
CREATE TABLE billing_versions (
 tenant_id BIGINT UNSIGNED NOT NULL,
 competition_id VARCHAR(255) NOT NULL,
 version BIGINT UNSIGNED NOT NULL DEFAULT 0,
 PRIMARY KEY (tenant_id, competition_id)
) ENGINE=InnoDB DEFAULT CHARACTER SET=utf8mb4;

CREATE TABLE billing_reports (
 tenant_id BIGINT UNSIGNED NOT NULL,
 competition_id VARCHAR(255) NOT NULL,
 finished_at BIGINT NOT NULL,
 version BIGINT UNSIGNED NOT NULL,
 player_count BIGINT NOT NULL,
 visitor_count BIGINT NOT NULL,
 PRIMARY KEY (tenant_id, competition_id)
) ENGINE=InnoDB DEFAULT CHARACTER SET=utf8mb4;
