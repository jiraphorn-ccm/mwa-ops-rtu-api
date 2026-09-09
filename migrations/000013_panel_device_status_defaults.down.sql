ALTER TABLE rtu.panel_devices
    ALTER COLUMN communication_status SET DEFAULT 'UNKNOWN';

ALTER TABLE rtu.panel_devices
    ALTER COLUMN health_status SET DEFAULT 'UNKNOWN';
