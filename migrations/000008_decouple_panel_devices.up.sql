-- Decouple panel_devices from device_models: equipment fields are snapshotted on
-- panel_devices at create/update time (master data in device_models is optional
-- copy source for the UI only).

-- ---------------------------------------------------------------------------
-- device_models — align equipment catalog fields with calibration_instruments
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.device_models
    ADD COLUMN equipment_type varchar(50),
    ADD COLUMN brand varchar(100),
    ADD COLUMN serial_number varchar(100),
    ADD COLUMN expire_date date;

CREATE INDEX idx_device_models_equipment_type ON rtu.device_models (equipment_type);

-- ---------------------------------------------------------------------------
-- panel_devices — owned equipment snapshot + calibration_date
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.panel_devices
    ADD COLUMN name varchar(100),
    ADD COLUMN equipment_type varchar(50),
    ADD COLUMN manufacturer varchar(100),
    ADD COLUMN brand varchar(100),
    ADD COLUMN model varchar(100),
    ADD COLUMN calibration_date date,
    ADD COLUMN expire_date date;

-- Backfill snapshot from linked device_models before dropping the FK.
UPDATE rtu.panel_devices pd
SET
    name             = COALESCE(pd.name, dm.name),
    equipment_type   = COALESCE(pd.equipment_type, dm.equipment_type),
    manufacturer     = COALESCE(pd.manufacturer, dm.manufacturer),
    brand            = COALESCE(pd.brand, dm.brand),
    model            = COALESCE(pd.model, dm.model),
    serial_number    = COALESCE(pd.serial_number, dm.serial_number),
    expire_date      = COALESCE(pd.expire_date, dm.expire_date)
FROM rtu.device_models dm
WHERE dm.id = pd.device_model_id;

UPDATE rtu.panel_devices
SET name = COALESCE(NULLIF(btrim(tag_name), ''), 'Unknown')
WHERE name IS NULL OR btrim(name) = '';

ALTER TABLE rtu.panel_devices ALTER COLUMN name SET NOT NULL;

-- Drop FK to device_models (no runtime link).
ALTER TABLE rtu.panel_devices DROP CONSTRAINT fk_panel_devices_device_model;
DROP INDEX IF EXISTS rtu.idx_panel_devices_device_model_id;
DROP INDEX IF EXISTS rtu.idx_panel_devices_panel_model;
ALTER TABLE rtu.panel_devices DROP COLUMN device_model_id;

CREATE INDEX idx_panel_devices_equipment_type ON rtu.panel_devices (equipment_type);
CREATE INDEX idx_panel_devices_name ON rtu.panel_devices (name);
