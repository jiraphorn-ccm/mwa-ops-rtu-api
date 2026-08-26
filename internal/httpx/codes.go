// Package httpx implements the shared MWA JSON response envelope: the success
// and error codes, the wire format, request binding and pagination helpers.
package httpx

import "net/http"

// Context groups a code into a use case, mirroring the `context` field of the
// response envelope.
const (
	CtxList            = "LIST"
	CtxDetail          = "DETAIL"
	CtxCreate          = "CREATE"
	CtxUpdate          = "UPDATE"
	CtxDelete          = "DELETE"
	CtxRestore         = "RESTORE"
	CtxStatus          = "STATUS"
	CtxSummary         = "SUMMARY"
	CtxValidation      = "VALIDATION"
	CtxAuth            = "AUTH"
	CtxDatabase        = "DATABASE"
	CtxSystem          = "SYSTEM"
	CtxPanel           = "PANEL"
	CtxDeviceModel     = "DEVICE_MODEL"
	CtxPanelDevice     = "PANEL_DEVICE"
	CtxInstrument      = "INSTRUMENT"
	CtxCalibration     = "CALIBRATION"
	CtxReading         = "CALIBRATION_READING"
	CtxWorkOrder       = "WORK_ORDER"
	CtxWorkOrderRound  = "WORK_ORDER_ROUND"
	CtxApproval        = "WO_APPROVAL"
	CtxActivityLog     = "WO_ACTIVITY_LOG"
	CtxPmReport        = "PM_REPORT"
	CtxChecklistItem   = "CHECKLIST_ITEM"
	CtxChecklistResult = "CHECKLIST_RESULT"
	CtxGroundTest      = "PM_GROUND_TEST"
	CtxPowerTest       = "PM_POWER_TEST"
	CtxCmReport        = "CM_REPORT"
	CtxAttachment      = "ATTACHMENT"
	CtxNotification    = "NOTIFICATION"
	CtxEngineer        = "ENGINEER"
	CtxProblemTopic    = "PROBLEM_TOPIC"
)

// SuccessCode is a `S*` envelope code together with its context, message and
// HTTP status.
type SuccessCode struct {
	Code    string
	Context string
	Message string
	Status  int
}

// CRUD / action success codes (S201_*) shared across MWA services.
var (
	SuccessList     = SuccessCode{"S201_001", CtxList, "Records fetched successfully.", http.StatusOK}
	SuccessDetail   = SuccessCode{"S201_002", CtxDetail, "Record fetched successfully.", http.StatusOK}
	SuccessCreate   = SuccessCode{"S201_003", CtxCreate, "Record created successfully.", http.StatusCreated}
	SuccessUpdate   = SuccessCode{"S201_004", CtxUpdate, "Record updated successfully.", http.StatusOK}
	SuccessDelete   = SuccessCode{"S201_005", CtxDelete, "Record deleted successfully.", http.StatusOK}
	SuccessRestore  = SuccessCode{"S201_007", CtxRestore, "Record restored successfully.", http.StatusOK}
	SuccessStatus   = SuccessCode{"S201_008", CtxStatus, "Status updated successfully.", http.StatusOK}
	SuccessSummary  = SuccessCode{"S201_009", CtxSummary, "Summary generated successfully.", http.StatusOK}
	SuccessBulkSave = SuccessCode{"S201_010", CtxCreate, "Records saved successfully.", http.StatusCreated}
)

// ErrorCode is an `E*` envelope code together with its context, default message
// and HTTP status.
type ErrorCode struct {
	Code    string
	Context string
	Message string
	Status  int
}

// Validation (E100_*).
var (
	ErrInvalidBody      = ErrorCode{"E100_000", CtxValidation, "Invalid request body.", http.StatusBadRequest}
	ErrInvalidID        = ErrorCode{"E100_001", CtxValidation, "Invalid ID parameter.", http.StatusBadRequest}
	ErrUnknownFields    = ErrorCode{"E100_002", CtxValidation, "Unknown fields in request.", http.StatusBadRequest}
	ErrValidationFailed = ErrorCode{"E100_003", CtxValidation, "Validation failed.", http.StatusBadRequest}
	ErrInvalidQuery     = ErrorCode{"E100_004", CtxValidation, "Invalid query parameters.", http.StatusBadRequest}
)

// Auth (E200_*). Reserved so this service stays wire-compatible with the other
// MWA services once it sits behind the shared gateway.
var (
	ErrTokenRequired      = ErrorCode{"E200_001", CtxAuth, "Authorization token is required.", http.StatusUnauthorized}
	ErrTokenMalformed     = ErrorCode{"E200_002", CtxAuth, "Invalid or malformed token.", http.StatusUnauthorized}
	ErrTokenExpired       = ErrorCode{"E200_003", CtxAuth, "Access token has expired.", http.StatusUnauthorized}
	ErrUnauthorized       = ErrorCode{"E200_004", CtxAuth, "Unauthorized.", http.StatusUnauthorized}
	ErrAccountDisabled    = ErrorCode{"E200_005", CtxAuth, "Account is disabled.", http.StatusForbidden}
	ErrInsufficientPerms  = ErrorCode{"E200_007", CtxAuth, "Insufficient permissions.", http.StatusForbidden}
	ErrInvalidCredentials = ErrorCode{"E300_001", CtxAuth, "Invalid credentials.", http.StatusUnauthorized}
)

// RTU business rules (E300_1xx). The range starts at 101 so it never collides
// with the E300_0xx codes already used by survey / repair / blowoff.
var (
	ErrPanelNotFound       = ErrorCode{"E300_101", CtxPanel, "Panel not found.", http.StatusNotFound}
	ErrPanelCodeExists     = ErrorCode{"E300_102", CtxPanel, "Panel code already exists.", http.StatusConflict}
	ErrPanelInactive       = ErrorCode{"E300_103", CtxPanel, "Panel is inactive.", http.StatusBadRequest}
	ErrPanelInUse          = ErrorCode{"E300_104", CtxPanel, "Cannot delete panel: devices are still attached.", http.StatusConflict}
	ErrDeviceModelNotFnd   = ErrorCode{"E300_105", CtxDeviceModel, "Device model not found.", http.StatusNotFound}
	ErrDeviceModelCodeDup  = ErrorCode{"E300_106", CtxDeviceModel, "Device model code already exists.", http.StatusConflict}
	ErrDeviceModelInUse    = ErrorCode{"E300_107", CtxDeviceModel, "Cannot delete device model: devices are still using it.", http.StatusConflict}
	ErrPanelDeviceNotFnd   = ErrorCode{"E300_108", CtxPanelDevice, "Panel device not found.", http.StatusNotFound}
	ErrDeviceSerialDup     = ErrorCode{"E300_109", CtxPanelDevice, "Device serial number already exists.", http.StatusConflict}
	ErrDeviceTagDup        = ErrorCode{"E300_110", CtxPanelDevice, "Tag name already exists in this panel.", http.StatusConflict}
	ErrDeviceInactive      = ErrorCode{"E300_111", CtxPanelDevice, "Panel device is inactive.", http.StatusBadRequest}
	ErrDeviceInUse         = ErrorCode{"E300_112", CtxPanelDevice, "Cannot delete panel device: calibrations reference it.", http.StatusConflict}
	ErrInstrumentNotFnd    = ErrorCode{"E300_113", CtxInstrument, "Calibration instrument not found.", http.StatusNotFound}
	ErrInstrumentSerialDup = ErrorCode{"E300_114", CtxInstrument, "Instrument serial number already exists.", http.StatusConflict}
	ErrInstrumentInactive  = ErrorCode{"E300_115", CtxInstrument, "Calibration instrument is inactive.", http.StatusBadRequest}
	ErrInstrumentExpired   = ErrorCode{"E300_116", CtxInstrument, "Calibration instrument certificate has expired.", http.StatusBadRequest}
	ErrInstrumentDates     = ErrorCode{"E300_117", CtxInstrument, "expire_date must be after calibration_date.", http.StatusBadRequest}
	ErrInstrumentInUse     = ErrorCode{"E300_118", CtxInstrument, "Cannot delete instrument: calibrations reference it.", http.StatusConflict}
	ErrCalibrationNotFnd   = ErrorCode{"E300_119", CtxCalibration, "Calibration not found.", http.StatusNotFound}
	ErrCalibrationResult   = ErrorCode{"E300_120", CtxCalibration, "result must be one of PASS, FAIL, ADJUSTED.", http.StatusBadRequest}
	ErrPerformedAtFuture   = ErrorCode{"E300_121", CtxCalibration, "performed_at cannot be in the future.", http.StatusBadRequest}
	ErrReadingNotFound     = ErrorCode{"E300_122", CtxReading, "Calibration reading not found.", http.StatusNotFound}
	ErrReadingSeqDup       = ErrorCode{"E300_123", CtxReading, "Duplicate reading sequence for this calibration.", http.StatusConflict}
	ErrDateRangeInvalid    = ErrorCode{"E300_124", CtxValidation, "The end of the range must be after its start.", http.StatusBadRequest}
	ErrPanelImageNotFnd    = ErrorCode{"E300_125", CtxPanel, "Panel image not found.", http.StatusNotFound}
	ErrImageTypeInvalid    = ErrorCode{"E300_126", CtxPanel, "image_type must be one of EXTERIOR, INTERIOR, DEVICE.", http.StatusBadRequest}
	ErrImageMimeInvalid    = ErrorCode{"E300_127", CtxPanel, "Unsupported image file type.", http.StatusBadRequest}
	ErrImageTooLarge       = ErrorCode{"E300_128", CtxPanel, "Image exceeds the maximum allowed size (10 MB).", http.StatusBadRequest}
	ErrS3NotConfigured     = ErrorCode{"E300_129", CtxPanel, "Object storage is not configured.", http.StatusServiceUnavailable}
	ErrDeviceNotInPanel    = ErrorCode{"E300_130", CtxPanelDevice, "Device does not belong to this panel.", http.StatusBadRequest}
)

// PM/CM report domain (E300_2xx).
var (
	ErrWorkOrderNotFnd              = ErrorCode{"E300_201", CtxWorkOrder, "Work order not found.", http.StatusNotFound}
	ErrWorkOrderNoDup               = ErrorCode{"E300_202", CtxWorkOrder, "Work order number already exists.", http.StatusConflict}
	ErrWorkOrderInactive            = ErrorCode{"E300_203", CtxWorkOrder, "Work order is cancelled.", http.StatusBadRequest}
	ErrWorkOrderStatusInvalid       = ErrorCode{"E300_204", CtxWorkOrder, "This action is not valid for the work order's current status.", http.StatusConflict}
	ErrPmScheduleTypeRequired       = ErrorCode{"E300_205", CtxWorkOrder, "pm_schedule_type is required when work_order_type is PM.", http.StatusBadRequest}
	ErrPmScheduleTypeNotAllowed     = ErrorCode{"E300_206", CtxWorkOrder, "pm_schedule_type must not be set when work_order_type is CM.", http.StatusBadRequest}
	ErrWorkOrderRoundNotFnd         = ErrorCode{"E300_207", CtxWorkOrderRound, "Work order round not found.", http.StatusNotFound}
	ErrRoundAlreadyCheckedIn        = ErrorCode{"E300_208", CtxWorkOrderRound, "This round has already checked in; reassignment requires a new round.", http.StatusConflict}
	ErrRoundNotCheckedIn            = ErrorCode{"E300_209", CtxWorkOrderRound, "Cannot check out before checking in.", http.StatusConflict}
	ErrRoundAlreadyCheckedOut       = ErrorCode{"E300_210", CtxWorkOrderRound, "This round has already checked out.", http.StatusConflict}
	ErrApprovalNotFnd               = ErrorCode{"E300_211", CtxApproval, "Approval record not found.", http.StatusNotFound}
	ErrApprovalRoundAlreadyReviewed = ErrorCode{"E300_212", CtxApproval, "This round has already been reviewed.", http.StatusConflict}
	ErrApprovalRepairDateRequired   = ErrorCode{"E300_213", CtxApproval, "repair_date is required when escalating to a CM work order.", http.StatusBadRequest}
	ErrApprovalNewWorkOrderRequired = ErrorCode{"E300_214", CtxApproval, "new_work_order_id is required when escalating to a CM work order.", http.StatusBadRequest}
	ErrPmReportNotFnd               = ErrorCode{"E300_215", CtxPmReport, "PM report not found.", http.StatusNotFound}
	ErrPmReportRoundDup             = ErrorCode{"E300_216", CtxPmReport, "This round already has a PM report.", http.StatusConflict}
	ErrPmReportNotDraft             = ErrorCode{"E300_217", CtxPmReport, "PM report has already been submitted.", http.StatusConflict}
	ErrChecklistItemNotFnd          = ErrorCode{"E300_218", CtxChecklistItem, "Checklist item not found.", http.StatusNotFound}
	ErrChecklistItemCodeDup         = ErrorCode{"E300_219", CtxChecklistItem, "Checklist item code already exists.", http.StatusConflict}
	ErrChecklistResultNotFnd        = ErrorCode{"E300_220", CtxChecklistResult, "Checklist result not found.", http.StatusNotFound}
	ErrChecklistResultDup           = ErrorCode{"E300_221", CtxChecklistResult, "This checklist item already has a result for this device.", http.StatusConflict}
	ErrGroundTestNotFnd             = ErrorCode{"E300_222", CtxGroundTest, "Ground test not found.", http.StatusNotFound}
	ErrGroundTestDup                = ErrorCode{"E300_223", CtxGroundTest, "This PM report already has a ground test.", http.StatusConflict}
	ErrPowerTestNotFnd              = ErrorCode{"E300_224", CtxPowerTest, "Power test not found.", http.StatusNotFound}
	ErrPowerTestDup                 = ErrorCode{"E300_225", CtxPowerTest, "This PM report already has a power test.", http.StatusConflict}
	ErrPowerTestPointNotFnd         = ErrorCode{"E300_226", CtxPowerTest, "Power test point not found.", http.StatusNotFound}
	ErrPowerTestPointRoleDup        = ErrorCode{"E300_227", CtxPowerTest, "This equipment role already has a test point.", http.StatusConflict}
	ErrCmReportNotFnd               = ErrorCode{"E300_228", CtxCmReport, "CM report not found.", http.StatusNotFound}
	ErrCmReportOriginRequired       = ErrorCode{"E300_229", CtxCmReport, "A CM report needs a work_order_id or a pm_report_id.", http.StatusBadRequest}
	ErrAttachmentNotFnd             = ErrorCode{"E300_230", CtxAttachment, "Attachment not found.", http.StatusNotFound}
	ErrAttachmentEntityInvalid      = ErrorCode{"E300_231", CtxAttachment, "entity_type must be one of WORK_ORDER, PM_REPORT, CM_REPORT, CALIBRATION, PM_GROUND_TEST, PM_POWER_TEST_POINT, PANEL_DEVICE.", http.StatusBadRequest}
	ErrNotificationNotFnd           = ErrorCode{"E300_232", CtxNotification, "Notification not found.", http.StatusNotFound}
	ErrEngineerNotFnd               = ErrorCode{"E300_233", CtxEngineer, "Engineer not found.", http.StatusNotFound}
	ErrCmReportRoundDup             = ErrorCode{"E300_234", CtxCmReport, "This round already has a CM report.", http.StatusConflict}
	ErrAttachmentTooLarge           = ErrorCode{"E300_235", CtxAttachment, "File exceeds the maximum allowed size (10 MB).", http.StatusBadRequest}
	ErrPmPowerTestRequired          = ErrorCode{"E300_236", CtxPmReport, "Power test is required for a 3-month PM before submit.", http.StatusBadRequest}
	ErrPmCalibrationRequired        = ErrorCode{"E300_237", CtxPmReport, "At least one calibration is required for a 6-month PM before submit.", http.StatusBadRequest}
	ErrCalibrationChannelInvalid    = ErrorCode{"E300_238", CtxCalibration, "channel_type must be one of PRESSURE, FLOW, LEVEL, RTU_READBACK.", http.StatusBadRequest}
	ErrCalibrationResultTypeInvalid = ErrorCode{"E300_239", CtxCalibration, "result_type must be one of TESTED, CALIBRATED_AND_TESTED, OTHER.", http.StatusBadRequest}
	ErrCalibrationLinkPmOnly        = ErrorCode{"E300_240", CtxCalibration, "work_order_id/pm_report_id may only link to a 6-month PM work order.", http.StatusBadRequest}
	ErrPmEscalateStatusInvalid      = ErrorCode{"E300_241", CtxCmReport, "Can only escalate an issue while the PM work order is in progress.", http.StatusConflict}
	ErrProblemTopicNotFnd           = ErrorCode{"E300_242", CtxProblemTopic, "Problem topic not found.", http.StatusNotFound}
	ErrProblemTopicCodeDup          = ErrorCode{"E300_243", CtxProblemTopic, "Problem topic code already exists.", http.StatusConflict}
	ErrProblemTopicInactive         = ErrorCode{"E300_244", CtxProblemTopic, "Problem topic is inactive.", http.StatusBadRequest}
	ErrProblemTopicInUse            = ErrorCode{"E300_245", CtxProblemTopic, "Cannot delete problem topic: CM reports reference it.", http.StatusConflict}
)

// Database / domain (E400_*).
var (
	ErrDuplicate  = ErrorCode{"E400_001", CtxDatabase, "Duplicate record.", http.StatusConflict}
	ErrNotFound   = ErrorCode{"E400_002", CtxDatabase, "Record not found.", http.StatusNotFound}
	ErrReferenced = ErrorCode{"E400_003", CtxDatabase, "Cannot delete: record is referenced.", http.StatusConflict}
)

// System (E500_*).
var (
	ErrInternal         = ErrorCode{"E500_001", CtxSystem, "Internal server error.", http.StatusInternalServerError}
	ErrEndpointNotFound = ErrorCode{"E500_002", CtxSystem, "Endpoint not found.", http.StatusNotFound}
	ErrSchemaOutdated   = ErrorCode{"E500_004", CtxSystem, "Database schema is outdated. Apply pending migrations before serving traffic.", http.StatusServiceUnavailable}
	ErrMethodNotAllowed = ErrorCode{"E500_005", CtxSystem, "Method not allowed.", http.StatusMethodNotAllowed}
	ErrPayloadTooLarge  = ErrorCode{"E500_006", CtxSystem, "Request payload is too large.", http.StatusRequestEntityTooLarge}
	ErrTimeout          = ErrorCode{"E500_007", CtxSystem, "Request timed out.", http.StatusGatewayTimeout}
	ErrDatabaseDown     = ErrorCode{"E500_008", CtxSystem, "Database is unavailable.", http.StatusServiceUnavailable}
)

// Staging guard (E600_*).
var ErrStagingGuard = ErrorCode{"E600_001", "STAGING_GUARD", "This operation is disabled on the Staging environment to protect production data.", http.StatusLocked}
