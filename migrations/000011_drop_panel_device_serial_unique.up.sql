-- panel_devices: serial_number is informational only — allow duplicates and empty values.
ALTER TABLE rtu.panel_devices
    DROP CONSTRAINT IF EXISTS uk_device_serial;
