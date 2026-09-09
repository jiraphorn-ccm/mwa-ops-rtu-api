ALTER TABLE rtu.panel_devices
    DROP COLUMN IF EXISTS output_range,
    DROP COLUMN IF EXISTS power_supply,
    DROP COLUMN IF EXISTS accuracy_class,
    DROP COLUMN IF EXISTS input_range;

ALTER TABLE rtu.device_models
    DROP COLUMN IF EXISTS output_range,
    DROP COLUMN IF EXISTS power_supply,
    DROP COLUMN IF EXISTS accuracy_class,
    DROP COLUMN IF EXISTS input_range;
