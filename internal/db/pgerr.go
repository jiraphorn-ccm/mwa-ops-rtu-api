package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rtu-api/internal/httpx"
)

// PostgreSQL SQLSTATE codes this service reacts to.
const (
	sqlStateUniqueViolation     = "23505"
	sqlStateForeignKeyViolation = "23503"
	sqlStateNotNullViolation    = "23502"
	sqlStateCheckViolation      = "23514"
	sqlStateExclusionViolation  = "23P01"
	sqlStateInvalidTextRepr     = "22P02"
	sqlStateNumericOutOfRange   = "22003"
	sqlStateStringTooLong       = "22001"
	sqlStateUndefinedTable      = "42P01"
	sqlStateUndefinedColumn     = "42703"
	sqlStateUndefinedFunction   = "42883"
	sqlStateInvalidSchemaName   = "3F000"
	sqlStateSerializationFail   = "40001"
	sqlStateDeadlock            = "40P01"
	sqlStateAdminShutdown       = "57P01"
	sqlStateCannotConnectNow    = "57P03"
	sqlStateQueryCanceled       = "57014"
	sqlStateInsufficientRes     = "53000"
	sqlStateTooManyConnections  = "53300"
)

// Constraints maps a PostgreSQL constraint name onto the business error the API
// should report, so callers get "Panel code already exists." rather than the
// generic "Duplicate record.".
type Constraints map[string]httpx.ErrorCode

// Options tunes the translation of a driver error for one call site.
type Options struct {
	// NotFound replaces the default E400_002 for pgx.ErrNoRows.
	NotFound *httpx.ErrorCode
	// Constraints maps constraint names to specific business errors.
	Constraints Constraints
}

// WithNotFound builds Options that report missing rows with a specific code.
func WithNotFound(code httpx.ErrorCode) Options {
	return Options{NotFound: &code}
}

// Translate converts a pgx/pgconn error into an *httpx.AppError. Errors that are
// already AppErrors pass through untouched, so services can return business
// errors through the same channel.
func Translate(err error, opts ...Options) error {
	if err == nil {
		return nil
	}

	var appErr *httpx.AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	opt := Options{}
	if len(opts) > 0 {
		opt = opts[0]
	}

	if errors.Is(err, pgx.ErrNoRows) {
		code := httpx.ErrNotFound
		if opt.NotFound != nil {
			code = *opt.NotFound
		}
		return httpx.Err(code).WithCause(err)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return httpx.Err(httpx.ErrTimeout).WithCause(err)
	}
	if errors.Is(err, context.Canceled) {
		return httpx.Err(httpx.ErrTimeout).WithCause(err)
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return httpx.Err(httpx.ErrInternal).WithCause(err)
	}

	if code, ok := opt.Constraints[pgErr.ConstraintName]; ok {
		return httpx.Err(code).WithCause(err)
	}

	switch pgErr.Code {
	case sqlStateUniqueViolation, sqlStateExclusionViolation:
		return httpx.Err(httpx.ErrDuplicate).
			WithField(columnOf(pgErr), httpx.IssueDuplicate, "This value is already taken.").
			WithCause(err)

	case sqlStateForeignKeyViolation:
		return httpx.Err(httpx.ErrReferenced).
			WithField(columnOf(pgErr), httpx.IssueNotFound, "The referenced record does not exist or is still in use.").
			WithCause(err)

	case sqlStateNotNullViolation:
		return httpx.Err(httpx.ErrValidationFailed).
			WithField(columnOf(pgErr), httpx.IssueRequired, "This field is required.").
			WithCause(err)

	case sqlStateCheckViolation:
		return httpx.Err(httpx.ErrValidationFailed).
			WithField(columnOf(pgErr), httpx.IssueInvalid, "The value violates constraint "+pgErr.ConstraintName+".").
			WithCause(err)

	case sqlStateInvalidTextRepr, sqlStateNumericOutOfRange, sqlStateStringTooLong:
		return httpx.Err(httpx.ErrValidationFailed).
			WithField(columnOf(pgErr), httpx.IssueInvalid, "The value has an invalid format or size.").
			WithCause(err)

	case sqlStateUndefinedTable, sqlStateUndefinedColumn, sqlStateUndefinedFunction, sqlStateInvalidSchemaName:
		return httpx.Err(httpx.ErrSchemaOutdated).WithCause(err)

	case sqlStateAdminShutdown, sqlStateCannotConnectNow, sqlStateInsufficientRes, sqlStateTooManyConnections:
		return httpx.Err(httpx.ErrDatabaseDown).WithCause(err)

	case sqlStateQueryCanceled:
		return httpx.Err(httpx.ErrTimeout).WithCause(err)

	case sqlStateSerializationFail, sqlStateDeadlock:
		return httpx.Err(httpx.ErrInternal).WithCause(err)

	default:
		return httpx.Err(httpx.ErrInternal).WithCause(err)
	}
}

// IsNotFound reports whether an error came from an empty result set.
func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IsUniqueViolation reports whether an error is a unique constraint violation,
// optionally restricted to a specific constraint name.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != sqlStateUniqueViolation {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// IsForeignKeyViolation reports whether an error is a foreign key violation,
// optionally restricted to a specific constraint name.
func IsForeignKeyViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != sqlStateForeignKeyViolation {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

func columnOf(pgErr *pgconn.PgError) string {
	if pgErr.ColumnName != "" {
		return pgErr.ColumnName
	}
	if pgErr.ConstraintName != "" {
		return pgErr.ConstraintName
	}
	return "body"
}
