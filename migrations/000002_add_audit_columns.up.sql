-- Audit columns: who created / last updated each row.
-- Values come from JWT user_id (or sub) when AUTH_ENABLED=true; nullable for
-- legacy rows and unauthenticated smoke tests.

ALTER TABLE rtu.panels
    ADD COLUMN created_by varchar(100),
    ADD COLUMN updated_by varchar(100);

ALTER TABLE rtu.device_models
    ADD COLUMN created_by varchar(100),
    ADD COLUMN updated_by varchar(100);

ALTER TABLE rtu.panel_devices
    ADD COLUMN created_by varchar(100),
    ADD COLUMN updated_by varchar(100);

ALTER TABLE rtu.calibration_instruments
    ADD COLUMN created_by varchar(100),
    ADD COLUMN updated_by varchar(100);

ALTER TABLE rtu.calibrations
    ADD COLUMN created_by varchar(100),
    ADD COLUMN updated_by varchar(100);

ALTER TABLE rtu.calibration_readings
    ADD COLUMN created_by varchar(100),
    ADD COLUMN updated_by varchar(100);
