// Package domain holds pure business rules with no I/O dependencies.
package domain

// Panel operational status values returned by the API (computed, not stored on panels).
const (
	PanelStatusNormal     = "NORMAL"
	PanelStatusMonitoring = "MONITORING"
	PanelStatusAbnormal   = "ABNORMAL"
)

// DeviceOperationalStatus maps stored communication/health columns to one
// operational tier used when aggregating panel status.
//
// ABNORMAL: critical health or offline communication.
// MONITORING: warning/degraded/unknown — needs attention but not hard failure.
// NORMAL: online and healthy.
func DeviceOperationalStatus(communicationStatus, healthStatus string) string {
	if healthStatus == "CRITICAL" || communicationStatus == "OFFLINE" {
		return PanelStatusAbnormal
	}
	if healthStatus == "WARNING" || communicationStatus == "DEGRADED" ||
		healthStatus == "UNKNOWN" || communicationStatus == "UNKNOWN" {
		return PanelStatusMonitoring
	}
	return PanelStatusNormal
}

// AggregatePanelStatus rolls up device operational statuses into a panel status.
// Priority: MONITORING beats ABNORMAL beats NORMAL (empty device list → NORMAL).
func AggregatePanelStatus(deviceStatuses []string) string {
	hasMonitoring, hasAbnormal := false, false
	for _, s := range deviceStatuses {
		switch s {
		case PanelStatusMonitoring:
			hasMonitoring = true
		case PanelStatusAbnormal:
			hasAbnormal = true
		}
	}
	if hasMonitoring {
		return PanelStatusMonitoring
	}
	if hasAbnormal {
		return PanelStatusAbnormal
	}
	return PanelStatusNormal
}
