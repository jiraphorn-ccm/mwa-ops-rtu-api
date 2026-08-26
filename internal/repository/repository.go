// Package repository is the only layer that talks to PostgreSQL.
//
// Writes and single-row lookups go through the sqlc-generated queries, which
// are verified against the schema at build time. List endpoints need a
// whitelisted ORDER BY and a variable WHERE clause, so their SQL is assembled
// here with bound parameters; nothing from the request is ever interpolated
// into the statement text.
package repository

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db/sqlc"
)

// Store bundles every repository behind a single dependency.
type Store struct {
	Pool                   *pgxpool.Pool
	Panels                 *PanelRepository
	DeviceModels           *DeviceModelRepository
	PanelDevices           *PanelDeviceRepository
	CalibrationInstruments *CalibrationInstrumentRepository
	Calibrations           *CalibrationRepository
	CalibrationReadings    *CalibrationReadingRepository
	PanelImages            *PanelImageRepository
	WorkOrders             *WorkOrderRepository
	WorkOrderRounds        *WorkOrderRoundRepository
	WorkOrderActivityLogs  *WorkOrderActivityLogRepository
	WoApprovals            *WoApprovalRepository
	Engineers              *EngineerRepository
	ChecklistItems         *ChecklistItemRepository
	ProblemTopics          *ProblemTopicRepository
	PmReports              *PmReportRepository
	CmReports              *CmReportRepository
	Attachments            *AttachmentRepository
	Notifications          *NotificationRepository
}

// New wires the repositories onto a connection pool.
func New(pool *pgxpool.Pool) *Store {
	q := sqlc.New(pool)
	return &Store{
		Pool:                   pool,
		Panels:                 &PanelRepository{pool: pool, q: q},
		DeviceModels:           &DeviceModelRepository{pool: pool, q: q},
		PanelDevices:           &PanelDeviceRepository{pool: pool, q: q},
		CalibrationInstruments: &CalibrationInstrumentRepository{pool: pool, q: q},
		Calibrations:           &CalibrationRepository{pool: pool, q: q},
		CalibrationReadings:    &CalibrationReadingRepository{pool: pool, q: q},
		PanelImages:            &PanelImageRepository{pool: pool, q: q},
		WorkOrders:             &WorkOrderRepository{pool: pool, q: q},
		WorkOrderRounds:        &WorkOrderRoundRepository{pool: pool, q: q},
		WorkOrderActivityLogs:  &WorkOrderActivityLogRepository{pool: pool, q: q},
		WoApprovals:            &WoApprovalRepository{pool: pool, q: q},
		Engineers:              &EngineerRepository{pool: pool, q: q},
		ChecklistItems:         &ChecklistItemRepository{pool: pool, q: q},
		ProblemTopics:          &ProblemTopicRepository{pool: pool, q: q},
		PmReports:              &PmReportRepository{pool: pool, q: q},
		CmReports:              &CmReportRepository{pool: pool, q: q},
		Attachments:            &AttachmentRepository{pool: pool, q: q},
		Notifications:          &NotificationRepository{pool: pool, q: q},
	}
}

// args accumulates bound parameters while a dynamic statement is built.
type args struct {
	values []any
}

// add binds a value and returns its placeholder.
func (a *args) add(v any) string {
	a.values = append(a.values, v)
	return "$" + strconv.Itoa(len(a.values))
}

// conditions collects the predicates of a WHERE clause.
type conditions []string

// where renders the collected predicates, or an always-true clause.
func (c conditions) where() string {
	if len(c) == 0 {
		return "TRUE"
	}
	return strings.Join(c, " AND ")
}

// likePattern turns user input into a safe `contains` pattern by neutralising
// the LIKE wildcards. Pair it with `ILIKE ... ESCAPE '\'`.
func likePattern(search string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(strings.TrimSpace(search)) + "%"
}
