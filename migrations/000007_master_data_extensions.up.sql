-- Extend calibration_instruments (equipment_type, brand) and add CM problem
-- topic master data linked from cm_reports.problem_topic_id.

-- ---------------------------------------------------------------------------
-- calibration_instruments — extra master fields for PM 6-month UI
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.calibration_instruments
    ADD COLUMN equipment_type varchar(50),
    ADD COLUMN brand varchar(100);

CREATE INDEX idx_calibration_instruments_equipment_type
    ON rtu.calibration_instruments (equipment_type);

-- ---------------------------------------------------------------------------
-- problem_topics — CM issue pills (System Design "หัวข้อปัญหา")
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.problem_topics (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        varchar(30) NOT NULL,
    name        varchar(255) NOT NULL,
    sort_order  smallint NOT NULL DEFAULT 0,
    active      boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    created_by  uuid,
    updated_by  uuid,
    CONSTRAINT uk_problem_topics_code UNIQUE (code)
);

CREATE INDEX idx_problem_topics_sort_order ON rtu.problem_topics (sort_order);
CREATE INDEX idx_problem_topics_active ON rtu.problem_topics (active);

CREATE TRIGGER trg_problem_topics_updated_at
    BEFORE UPDATE ON rtu.problem_topics
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

INSERT INTO rtu.problem_topics (code, name, sort_order) VALUES
    ('COMM_LOST',       'Communication Lost', 1),
    ('POWER_FAILURE',   'Power Failure',      2),
    ('PRESSURE_ERROR',  'Pressure Error',     3),
    ('SENSOR_FAILURE',  'Sensor Failure',     4),
    ('FLOW_ERROR',      'Flow Error',         5),
    ('PLC_FAULT',       'PLC Fault',          6),
    ('BREAKER_TRIP',    'Breaker Trip',       7),
    ('BATTERY_LOW',     'Battery Low',        8),
    ('CABINET_DAMAGE',  'Cabinet Damage',     9),
    ('CABLE_BROKEN',    'Cable Broken',       10),
    ('GROUND_FAULT',    'Ground Fault',       11),
    ('OTHERS',          'Others',             12);

-- ---------------------------------------------------------------------------
-- cm_reports — FK to problem_topics (tag_code kept for legacy reads)
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.cm_reports
    ADD COLUMN problem_topic_id uuid,
    ADD CONSTRAINT fk_cm_reports_problem_topic
        FOREIGN KEY (problem_topic_id)
        REFERENCES rtu.problem_topics (id)
        ON UPDATE CASCADE ON DELETE RESTRICT;

CREATE INDEX idx_cm_reports_problem_topic_id ON rtu.cm_reports (problem_topic_id);
