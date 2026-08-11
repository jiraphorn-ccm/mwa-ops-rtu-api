package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// CalibrationReadingRepository reads and writes rtu.calibration_readings.
type CalibrationReadingRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var readingConstraints = db.Constraints{
	"uk_calibration_reading_sequence":     httpx.ErrReadingSeqDup,
	"fk_calibration_readings_calibration": httpx.ErrCalibrationNotFnd,
	"ck_calibration_readings_sequence":    httpx.ErrValidationFailed,
}

// ListByCalibration returns every reading of one calibration, ordered by
// sequence.
func (r *CalibrationReadingRepository) ListByCalibration(ctx context.Context, calibrationID uuid.UUID) ([]sqlc.CalibrationReading, error) {
	readings, err := r.q.ListCalibrationReadings(ctx, calibrationID)
	if err != nil {
		return nil, db.Translate(err)
	}
	return readings, nil
}

// ListByCalibrations returns the readings of several calibrations in one query,
// so a list endpoint never runs into an N+1.
func (r *CalibrationReadingRepository) ListByCalibrations(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]sqlc.CalibrationReading, error) {
	grouped := make(map[uuid.UUID][]sqlc.CalibrationReading, len(ids))
	if len(ids) == 0 {
		return grouped, nil
	}

	readings, err := r.q.ListCalibrationReadingsForCalibrations(ctx, ids)
	if err != nil {
		return nil, db.Translate(err)
	}
	for _, reading := range readings {
		grouped[reading.CalibrationID] = append(grouped[reading.CalibrationID], reading)
	}
	return grouped, nil
}

// Get returns a single reading by id.
func (r *CalibrationReadingRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.CalibrationReading, error) {
	reading, err := r.q.GetCalibrationReading(ctx, id)
	if err != nil {
		return sqlc.CalibrationReading{}, db.Translate(err, db.WithNotFound(httpx.ErrReadingNotFound))
	}
	return reading, nil
}

// GetForCalibration returns a reading scoped to its parent calibration.
func (r *CalibrationReadingRepository) GetForCalibration(ctx context.Context, calibrationID, id uuid.UUID) (sqlc.CalibrationReading, error) {
	reading, err := r.q.GetCalibrationReadingForCalibration(ctx, sqlc.GetCalibrationReadingForCalibrationParams{
		ID: id, CalibrationID: calibrationID,
	})
	if err != nil {
		return sqlc.CalibrationReading{}, db.Translate(err, db.WithNotFound(httpx.ErrReadingNotFound))
	}
	return reading, nil
}

// Create appends one reading to a calibration.
func (r *CalibrationReadingRepository) Create(ctx context.Context, arg sqlc.CreateCalibrationReadingParams) (sqlc.CalibrationReading, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	reading, err := r.q.CreateCalibrationReading(ctx, arg)
	if err != nil {
		return sqlc.CalibrationReading{}, db.Translate(err, db.Options{Constraints: readingConstraints})
	}
	return reading, nil
}

// NextSequence returns the sequence number a new reading should take.
func (r *CalibrationReadingRepository) NextSequence(ctx context.Context, calibrationID uuid.UUID) (int16, error) {
	next, err := r.q.NextCalibrationReadingSequence(ctx, calibrationID)
	if err != nil {
		return 0, db.Translate(err)
	}
	return next, nil
}

// Update applies a partial update to a reading.
func (r *CalibrationReadingRepository) Update(ctx context.Context, arg sqlc.UpdateCalibrationReadingParams) (sqlc.CalibrationReading, error) {
	arg.UpdatedBy = updateAudit(ctx)
	reading, err := r.q.UpdateCalibrationReading(ctx, arg)
	if err != nil {
		return sqlc.CalibrationReading{}, db.Translate(err, db.Options{
			NotFound:    &httpx.ErrReadingNotFound,
			Constraints: readingConstraints,
		})
	}
	return reading, nil
}

// Delete removes a reading permanently.
func (r *CalibrationReadingRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteCalibrationReading(ctx, id)
	if err != nil {
		return db.Translate(err)
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrReadingNotFound)
	}
	return nil
}
