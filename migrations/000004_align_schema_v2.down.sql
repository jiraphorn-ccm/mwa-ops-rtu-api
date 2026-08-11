-- Revert schema v2 alignment.

DO $$
BEGIN
    IF to_regclass('rtu.panel_images') IS NOT NULL THEN
        EXECUTE 'DROP INDEX IF EXISTS rtu.idx_panel_images_panel_type';
        EXECUTE 'ALTER INDEX IF EXISTS rtu.idx_panel_images_panel_id RENAME TO idx_panel_images_panel';
        EXECUTE 'ALTER INDEX IF EXISTS rtu.idx_panel_images_panel_sort RENAME TO idx_panel_images_sort';

        EXECUTE 'ALTER TABLE rtu.panel_images DROP CONSTRAINT IF EXISTS ck_panel_images_device_type';
        EXECUTE 'ALTER TABLE rtu.panel_images DROP CONSTRAINT IF EXISTS fk_panel_images_device';
        EXECUTE 'ALTER TABLE rtu.panel_images ADD COLUMN IF NOT EXISTS panel_device_id uuid';
        EXECUTE $sql$
            ALTER TABLE rtu.panel_images
                ADD CONSTRAINT fk_panel_images_device FOREIGN KEY (panel_device_id)
                    REFERENCES rtu.panel_devices (id) ON UPDATE CASCADE ON DELETE RESTRICT
        $sql$;
        EXECUTE $sql$
            ALTER TABLE rtu.panel_images
                ADD CONSTRAINT ck_panel_images_device_type CHECK (
                    (image_type = 'DEVICE' AND panel_device_id IS NOT NULL)
                    OR (image_type IN ('EXTERIOR', 'INTERIOR') AND panel_device_id IS NULL)
                )
        $sql$;
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_panel_images_device ON rtu.panel_images (panel_device_id) WHERE panel_device_id IS NOT NULL';

        EXECUTE 'ALTER TABLE rtu.panel_images ALTER COLUMN created_by TYPE varchar(100) USING created_by::text';
        EXECUTE 'ALTER TABLE rtu.panel_images ALTER COLUMN updated_by TYPE varchar(100) USING updated_by::text';
    END IF;
END;
$$;

DROP TRIGGER IF EXISTS trg_calibration_readings_updated_at ON rtu.calibration_readings;
ALTER TABLE rtu.calibration_readings DROP COLUMN IF EXISTS updated_at;

ALTER TABLE rtu.calibration_readings
    ALTER COLUMN created_by TYPE varchar(100) USING created_by::text,
    ALTER COLUMN updated_by TYPE varchar(100) USING updated_by::text;

ALTER TABLE rtu.calibrations
    ALTER COLUMN created_by TYPE varchar(100) USING created_by::text,
    ALTER COLUMN updated_by TYPE varchar(100) USING updated_by::text;

ALTER TABLE rtu.calibration_instruments
    ALTER COLUMN created_by TYPE varchar(100) USING created_by::text,
    ALTER COLUMN updated_by TYPE varchar(100) USING updated_by::text;

ALTER TABLE rtu.panel_devices
    ALTER COLUMN created_by TYPE varchar(100) USING created_by::text,
    ALTER COLUMN updated_by TYPE varchar(100) USING updated_by::text;

ALTER TABLE rtu.device_models
    ALTER COLUMN created_by TYPE varchar(100) USING created_by::text,
    ALTER COLUMN updated_by TYPE varchar(100) USING updated_by::text;

ALTER TABLE rtu.panels
    ALTER COLUMN created_by TYPE varchar(100) USING created_by::text,
    ALTER COLUMN updated_by TYPE varchar(100) USING updated_by::text;

DROP INDEX IF EXISTS rtu.idx_panels_status;
ALTER TABLE rtu.panels DROP CONSTRAINT IF EXISTS ck_panels_status;
ALTER TABLE rtu.panels DROP COLUMN IF EXISTS status;
