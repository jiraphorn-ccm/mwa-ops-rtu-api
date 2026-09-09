-- New panel devices default to ONLINE + NORMAL (operational_status NORMAL).
ALTER TABLE rtu.panel_devices
    ALTER COLUMN communication_status SET DEFAULT 'ONLINE';

ALTER TABLE rtu.panel_devices
    ALTER COLUMN health_status SET DEFAULT 'NORMAL';
