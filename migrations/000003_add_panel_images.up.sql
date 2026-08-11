-- Panel and device photos stored in S3; metadata lives here.

CREATE TABLE rtu.panel_images (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    panel_id        uuid NOT NULL,
    panel_device_id uuid,
    image_type      varchar(20) NOT NULL,
    s3_bucket       varchar(100) NOT NULL,
    s3_key          varchar(500) NOT NULL,
    original_name   varchar(255),
    mime_type       varchar(100) NOT NULL,
    file_size       bigint NOT NULL,
    caption         text,
    sort_order      smallint NOT NULL DEFAULT 0,
    active          boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    created_by      varchar(100),
    updated_by      varchar(100),
    CONSTRAINT fk_panel_images_panel FOREIGN KEY (panel_id)
        REFERENCES rtu.panels (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_panel_images_device FOREIGN KEY (panel_device_id)
        REFERENCES rtu.panel_devices (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT ck_panel_images_type CHECK (image_type IN ('EXTERIOR', 'INTERIOR', 'DEVICE')),
    CONSTRAINT ck_panel_images_device_type CHECK (
        (image_type = 'DEVICE' AND panel_device_id IS NOT NULL)
        OR (image_type IN ('EXTERIOR', 'INTERIOR') AND panel_device_id IS NULL)
    ),
    CONSTRAINT ck_panel_images_file_size CHECK (file_size > 0 AND file_size <= 10485760)
);

CREATE INDEX idx_panel_images_panel ON rtu.panel_images (panel_id);
CREATE INDEX idx_panel_images_device ON rtu.panel_images (panel_device_id)
    WHERE panel_device_id IS NOT NULL;
CREATE INDEX idx_panel_images_active ON rtu.panel_images (active);
CREATE INDEX idx_panel_images_sort ON rtu.panel_images (panel_id, sort_order);

CREATE TRIGGER trg_panel_images_updated_at
    BEFORE UPDATE ON rtu.panel_images
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();
