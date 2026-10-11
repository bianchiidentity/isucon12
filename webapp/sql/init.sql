DELETE FROM billing_reports;
DELETE FROM billing_versions;
DELETE FROM tenant WHERE id > 100;
-- With per-player MIN timestamps, baseline visits remain unchanged by later visits.
-- Groups containing only post-baseline visits are removed. No reaggregation needed.
DELETE FROM visit_history WHERE created_at >= '1654041600';
UPDATE id_generator SET id=2678400000 WHERE stub='a';
ALTER TABLE id_generator AUTO_INCREMENT=2678400000;
