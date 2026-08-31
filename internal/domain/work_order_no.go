package domain

import (
	"fmt"
	"strconv"
)

// FormatWorkOrderNo builds the system-generated work order number:
// {TYPE}-{panelCode}-{sequence}, e.g. PM-RTU-011-0001.
//
// Sequences 1–9999 are zero-padded to 4 digits; from 10000 onward the full
// integer is used (10001, 10002, …).
func FormatWorkOrderNo(workOrderType, panelCode string, sequence int64) string {
	return fmt.Sprintf("%s-%s-%s", workOrderType, panelCode, formatWorkOrderSequence(sequence))
}

func formatWorkOrderSequence(sequence int64) string {
	if sequence < 10_000 {
		return fmt.Sprintf("%04d", sequence)
	}
	return strconv.FormatInt(sequence, 10)
}
