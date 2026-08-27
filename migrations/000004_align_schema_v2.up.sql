-- Align schema to v2:
--   panels.status
--   audit columns: varchar(100) -> uuid
--   calibration_readings.updated_at
--   panel_images: drop panel_device_id (panel-only reference)

-- ---------------------------------------------------------------------------
-- panels.status
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.panels
    ADD COLUMN status varchar(20);

ALTER TABLE rtu.panels
    ADD CONSTRAINT ck_panels_status
        CHECK (status IS NULL OR status IN ('NORMAL', 'ABNORMAL', 'PENDING_REVIEW'));

CREATE INDEX idx_panels_status ON rtu.panels (status);

-- ---------------------------------------------------------------------------
-- audit: created_by / updated_by -> uuid
-- Non-UUID legacy values become NULL.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION rtu.try_parse_uuid(input text) RETURNS uuid
LANGUAGE plpgsql IMMUTABLE AS $$
BEGIN
    IF input IS NULL OR btrim(input) = '' THEN
        RETURN NULL;
    END IF;
    IF input ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
        RETURN input::uuid;
    END IF;
    RETURN NULL;
END;
$$;

ALTER TABLE rtu.panels
    ALTER COLUMN created_by TYPE uuid USING rtu.try_parse_uuid(created_by),
    ALTER COLUMN updated_by TYPE uuid USING rtu.try_parse_uuid(updated_by);

ALTER TABLE rtu.device_models
    ALTER COLUMN created_by TYPE uuid USING rtu.try_parse_uuid(created_by),
    ALTER COLUMN updated_by TYPE uuid USING rtu.try_parse_uuid(updated_by);

ALTER TABLE rtu.panel_devices
    ALTER COLUMN created_by TYPE uuid USING rtu.try_parse_uuid(created_by),
    ALTER COLUMN updated_by TYPE uuid USING rtu.try_parse_uuid(updated_by);

ALTER TABLE rtu.calibration_instruments
    ALTER COLUMN created_by TYPE uuid USING rtu.try_parse_uuid(created_by),
    ALTER COLUMN updated_by TYPE uuid USING rtu.try_parse_uuid(updated_by);

ALTER TABLE rtu.calibrations
    ALTER COLUMN created_by TYPE uuid USING rtu.try_parse_uuid(created_by),
    ALTER COLUMN updated_by TYPE uuid USING rtu.try_parse_uuid(updated_by);

ALTER TABLE rtu.calibration_readings
    ALTER COLUMN created_by TYPE uuid USING rtu.try_parse_uuid(created_by),
    ALTER COLUMN updated_by TYPE uuid USING rtu.try_parse_uuid(updated_by);

ALTER TABLE rtu.panel_images
    ALTER COLUMN created_by TYPE uuid USING rtu.try_parse_uuid(created_by),
    ALTER COLUMN updated_by TYPE uuid USING rtu.try_parse_uuid(updated_by);

DROP FUNCTION rtu.try_parse_uuid(text);

-- ---------------------------------------------------------------------------
-- calibration_readings.updated_at
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.calibration_readings
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

CREATE TRIGGER trg_calibration_readings_updated_at
    BEFORE UPDATE ON rtu.calibration_readings
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- panel_images: panel-only (no panel_device_id)
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.panel_images DROP CONSTRAINT IF EXISTS fk_panel_images_device;
ALTER TABLE rtu.panel_images DROP CONSTRAINT IF EXISTS ck_panel_images_device_type;
DROP INDEX IF EXISTS rtu.idx_panel_images_device;
ALTER TABLE rtu.panel_images DROP COLUMN IF EXISTS panel_device_id;

ALTER INDEX IF EXISTS rtu.idx_panel_images_panel RENAME TO idx_panel_images_panel_id;
ALTER INDEX IF EXISTS rtu.idx_panel_images_sort RENAME TO idx_panel_images_panel_sort;

CREATE INDEX IF NOT EXISTS idx_panel_images_panel_type ON rtu.panel_images (panel_id, image_type);
