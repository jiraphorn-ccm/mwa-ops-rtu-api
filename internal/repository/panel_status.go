package repository

// panelStatusExprSQL computes panel operational status for the panel aliased as p.
// Logic mirrors domain.AggregatePanelStatus + domain.DeviceOperationalStatus.
const panelStatusExprSQL = `
CASE
  WHEN EXISTS (
    SELECT 1
    FROM rtu.panel_devices pd
    WHERE pd.panel_id = p.id
      AND pd.active
      AND (
        pd.health_status = 'WARNING'
        OR pd.communication_status = 'DEGRADED'
        OR pd.health_status = 'UNKNOWN'
        OR pd.communication_status = 'UNKNOWN'
      )
  ) THEN 'MONITORING'
  WHEN EXISTS (
    SELECT 1
    FROM rtu.panel_devices pd
    WHERE pd.panel_id = p.id
      AND pd.active
      AND (
        pd.health_status = 'CRITICAL'
        OR pd.communication_status = 'OFFLINE'
      )
  ) THEN 'ABNORMAL'
  ELSE 'NORMAL'
END`
