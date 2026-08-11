ALTER TABLE rtu.panels
    ADD COLUMN status varchar(20);

ALTER TABLE rtu.panels
    ADD CONSTRAINT ck_panels_status
        CHECK (status IS NULL OR status IN ('NORMAL', 'ABNORMAL', 'PENDING_REVIEW'));

CREATE INDEX idx_panels_status ON rtu.panels (status);
