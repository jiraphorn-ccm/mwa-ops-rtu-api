ALTER TABLE rtu.cm_reports
    DROP CONSTRAINT IF EXISTS fk_cm_reports_problem_topic,
    DROP COLUMN IF EXISTS problem_topic_id;

DROP TRIGGER IF EXISTS trg_problem_topics_updated_at ON rtu.problem_topics;
DROP TABLE IF EXISTS rtu.problem_topics;

DROP INDEX IF EXISTS rtu.idx_calibration_instruments_equipment_type;

ALTER TABLE rtu.calibration_instruments
    DROP COLUMN IF EXISTS equipment_type,
    DROP COLUMN IF EXISTS brand;
