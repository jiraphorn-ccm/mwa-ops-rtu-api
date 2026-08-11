-- RTU calibration domain schema.
-- Requires PostgreSQL >= 14 (gen_random_uuid() is built in since 13).

CREATE SCHEMA IF NOT EXISTS rtu;

CREATE OR REPLACE FUNCTION rtu.set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- panels
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.panels (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       varchar(20) NOT NULL,
    location   text,
    latitude   numeric(10, 7),
    longitude  numeric(10, 7),
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_panels_code UNIQUE (code),
    CONSTRAINT ck_panels_latitude CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    CONSTRAINT ck_panels_longitude CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180)
);

CREATE INDEX idx_panels_active ON rtu.panels (active);
CREATE INDEX idx_panels_created_at ON rtu.panels (created_at DESC);

CREATE TRIGGER trg_panels_updated_at
    BEFORE UPDATE ON rtu.panels
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- device_models
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.device_models (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code         varchar(30) NOT NULL,
    name         varchar(100) NOT NULL,
    manufacturer varchar(100),
    model        varchar(100),
    description  text,
    active       boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_device_model_code UNIQUE (code)
);

CREATE INDEX idx_device_models_active ON rtu.device_models (active);

CREATE TRIGGER trg_device_models_updated_at
    BEFORE UPDATE ON rtu.device_models
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- panel_devices
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.panel_devices (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    panel_id             uuid NOT NULL,
    device_model_id      uuid NOT NULL,
    tag_name             varchar(100),
    serial_number        varchar(100),
    asset_code           varchar(100),
    firmware_version     varchar(50),
    communication_status varchar(20) NOT NULL DEFAULT 'UNKNOWN',
    health_status        varchar(20) NOT NULL DEFAULT 'UNKNOWN',
    installed_at         date,
    last_seen_at         timestamptz,
    note                 text,
    active               boolean NOT NULL DEFAULT true,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_panel_devices_panel FOREIGN KEY (panel_id)
        REFERENCES rtu.panels (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_panel_devices_device_model FOREIGN KEY (device_model_id)
        REFERENCES rtu.device_models (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uk_device_serial UNIQUE (serial_number),
    CONSTRAINT ck_panel_devices_communication_status
        CHECK (communication_status IN ('ONLINE', 'OFFLINE', 'DEGRADED', 'UNKNOWN')),
    CONSTRAINT ck_panel_devices_health_status
        CHECK (health_status IN ('NORMAL', 'WARNING', 'CRITICAL', 'UNKNOWN'))
);

CREATE INDEX idx_panel_devices_panel_id ON rtu.panel_devices (panel_id);
CREATE INDEX idx_panel_devices_device_model_id ON rtu.panel_devices (device_model_id);
CREATE INDEX idx_panel_devices_panel_model ON rtu.panel_devices (panel_id, device_model_id);
CREATE INDEX idx_panel_devices_active ON rtu.panel_devices (active);
CREATE INDEX idx_panel_devices_last_seen_at ON rtu.panel_devices (last_seen_at DESC NULLS LAST);

-- A tag name identifies a device inside its own panel, so it only has to be
-- unique per panel rather than globally.
CREATE UNIQUE INDEX uk_panel_device_tag ON rtu.panel_devices (panel_id, tag_name)
    WHERE tag_name IS NOT NULL;

CREATE TRIGGER trg_panel_devices_updated_at
    BEFORE UPDATE ON rtu.panel_devices
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- calibration_instruments
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.calibration_instruments (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             varchar(100) NOT NULL,
    manufacturer     varchar(100),
    model            varchar(100),
    serial_number    varchar(100),
    calibration_date date,
    expire_date      date,
    active           boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_instrument_serial UNIQUE (serial_number),
    CONSTRAINT ck_instrument_expire_after_calibration
        CHECK (expire_date IS NULL OR calibration_date IS NULL OR expire_date > calibration_date)
);

CREATE INDEX idx_calibration_instruments_active ON rtu.calibration_instruments (active);
CREATE INDEX idx_calibration_instruments_expire_date ON rtu.calibration_instruments (expire_date);

CREATE TRIGGER trg_calibration_instruments_updated_at
    BEFORE UPDATE ON rtu.calibration_instruments
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- calibrations
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.calibrations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    panel_device_id uuid NOT NULL,
    instrument_id   uuid NOT NULL,
    performed_by    varchar(100),
    performed_at    timestamptz NOT NULL,
    result          varchar(20) NOT NULL,
    remark          text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_calibrations_panel_device FOREIGN KEY (panel_device_id)
        REFERENCES rtu.panel_devices (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_calibrations_instrument FOREIGN KEY (instrument_id)
        REFERENCES rtu.calibration_instruments (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT ck_calibrations_result CHECK (result IN ('PASS', 'FAIL', 'ADJUSTED'))
);

CREATE INDEX idx_calibrations_panel_device_id ON rtu.calibrations (panel_device_id);
CREATE INDEX idx_calibrations_instrument_id ON rtu.calibrations (instrument_id);
CREATE INDEX idx_calibrations_performed_at ON rtu.calibrations (performed_at DESC);
CREATE INDEX idx_calibrations_device_performed_at
    ON rtu.calibrations (panel_device_id, performed_at DESC);

CREATE TRIGGER trg_calibrations_updated_at
    BEFORE UPDATE ON rtu.calibrations
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- calibration_readings
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.calibration_readings (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    calibration_id uuid NOT NULL,
    sequence       smallint NOT NULL,
    item_label     varchar(150),
    parameter_key  varchar(50) NOT NULL,
    value          numeric,
    unit           varchar(20),
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_calibration_readings_calibration FOREIGN KEY (calibration_id)
        REFERENCES rtu.calibrations (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT uk_calibration_reading_sequence UNIQUE (calibration_id, sequence),
    CONSTRAINT ck_calibration_readings_sequence CHECK (sequence > 0)
);

CREATE INDEX idx_calibration_readings_calibration_id ON rtu.calibration_readings (calibration_id);
CREATE INDEX idx_calibration_readings_parameter_key ON rtu.calibration_readings (parameter_key);
