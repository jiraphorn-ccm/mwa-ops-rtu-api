ALTER TABLE rtu.calibration_readings
    DROP COLUMN IF EXISTS updated_by,
    DROP COLUMN IF EXISTS created_by;

ALTER TABLE rtu.calibrations
    DROP COLUMN IF EXISTS updated_by,
    DROP COLUMN IF EXISTS created_by;

ALTER TABLE rtu.calibration_instruments
    DROP COLUMN IF EXISTS updated_by,
    DROP COLUMN IF EXISTS created_by;

ALTER TABLE rtu.panel_devices
    DROP COLUMN IF EXISTS updated_by,
    DROP COLUMN IF EXISTS created_by;

ALTER TABLE rtu.device_models
    DROP COLUMN IF EXISTS updated_by,
    DROP COLUMN IF EXISTS created_by;

ALTER TABLE rtu.panels
    DROP COLUMN IF EXISTS updated_by,
    DROP COLUMN IF EXISTS created_by;
