-- Panel operational status is derived from active panel_devices at query time.
-- Priority: MONITORING > ABNORMAL > NORMAL (see internal/domain/panel_status.go).

DROP INDEX IF EXISTS rtu.idx_panels_status;

ALTER TABLE rtu.panels DROP CONSTRAINT IF EXISTS ck_panels_status;

ALTER TABLE rtu.panels DROP COLUMN IF EXISTS status;
