-- Re-link panel_devices to device_models (best-effort: match by name+model code path).

ALTER TABLE rtu.panel_devices
    ADD COLUMN device_model_id uuid;

-- Pick the first active model with matching name when possible.
UPDATE rtu.panel_devices pd
SET device_model_id = (
    SELECT dm.id
    FROM rtu.device_models dm
    WHERE dm.name = pd.name
      AND (pd.model IS NULL OR dm.model IS NULL OR dm.model = pd.model)
    ORDER BY dm.active DESC, dm.created_at
    LIMIT 1
);

UPDATE rtu.panel_devices pd
SET device_model_id = (SELECT id FROM rtu.device_models ORDER BY created_at LIMIT 1)
WHERE device_model_id IS NULL
  AND EXISTS (SELECT 1 FROM rtu.device_models);

ALTER TABLE rtu.panel_devices
    ALTER COLUMN device_model_id SET NOT NULL;

ALTER TABLE rtu.panel_devices
    ADD CONSTRAINT fk_panel_devices_device_model FOREIGN KEY (device_model_id)
        REFERENCES rtu.device_models (id) ON UPDATE CASCADE ON DELETE RESTRICT;

CREATE INDEX idx_panel_devices_device_model_id ON rtu.panel_devices (device_model_id);
CREATE INDEX idx_panel_devices_panel_model ON rtu.panel_devices (panel_id, device_model_id);

DROP INDEX IF EXISTS rtu.idx_panel_devices_equipment_type;
DROP INDEX IF EXISTS rtu.idx_panel_devices_name;

ALTER TABLE rtu.panel_devices
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS equipment_type,
    DROP COLUMN IF EXISTS manufacturer,
    DROP COLUMN IF EXISTS brand,
    DROP COLUMN IF EXISTS model,
    DROP COLUMN IF EXISTS calibration_date,
    DROP COLUMN IF EXISTS expire_date;

ALTER TABLE rtu.device_models
    DROP COLUMN IF EXISTS equipment_type,
    DROP COLUMN IF EXISTS brand,
    DROP COLUMN IF EXISTS serial_number,
    DROP COLUMN IF EXISTS expire_date;

DROP INDEX IF EXISTS rtu.idx_device_models_equipment_type;
