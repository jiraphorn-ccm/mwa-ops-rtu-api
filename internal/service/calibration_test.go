package service

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/httpx"
)

func TestBuildReadingRowsAutoSequence(t *testing.T) {
	rows, err := buildReadingRows([]CalibrationReadingInput{
		{ParameterKey: "pressure", Value: decPtr("1.0")},
		{ParameterKey: "temperature", Value: decPtr("25.0")},
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 || rows[0].Sequence != 1 || rows[1].Sequence != 2 {
		t.Fatalf("sequences=%v", rows)
	}
}

func TestBuildReadingRowsRejectsDuplicateSequence(t *testing.T) {
	seq := int16(1)
	_, err := buildReadingRows([]CalibrationReadingInput{
		{Sequence: &seq, ParameterKey: "a"},
		{Sequence: &seq, ParameterKey: "b"},
	}, 0)
	if err == nil {
		t.Fatal("expected duplicate sequence error")
	}
	var appErr *httpx.AppError
	if !errors.As(err, &appErr) || appErr.Code.Code != httpx.ErrReadingSeqDup.Code {
		t.Fatalf("got %v", err)
	}
}

func TestCheckPerformedAtRejectsFuture(t *testing.T) {
	svc := &CalibrationService{}
	err := svc.checkPerformedAt(time.Now().Add(24 * time.Hour))
	if err == nil {
		t.Fatal("expected future performed_at to fail")
	}
}

func TestValidateCoordinates(t *testing.T) {
	badLat := decimal.NewFromInt(999)
	if err := validateCoordinates(&badLat, nil); err == nil {
		t.Fatal("expected latitude validation error")
	}
}

func decPtr(s string) *decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return &d
}
