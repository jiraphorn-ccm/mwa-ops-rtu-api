-- Restore global uniqueness (fails if duplicate serial_number values exist).
ALTER TABLE rtu.panel_devices
    ADD CONSTRAINT uk_device_serial UNIQUE (serial_number);
