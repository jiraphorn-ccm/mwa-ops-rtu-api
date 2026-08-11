package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// FieldSet records which top-level keys the client actually sent, so a PATCH
// can tell "field omitted" apart from "field explicitly set to null".
type FieldSet map[string]struct{}

// Has reports whether the client sent the given JSON key.
func (f FieldSet) Has(name string) bool {
	_, ok := f[name]
	return ok
}

// Bind decodes and validates a JSON request body and reports which keys were
// present. An empty body is treated as `{}` so PATCH with no changes is legal.
func Bind(r *http.Request, dst any) (FieldSet, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			return nil, Err(ErrPayloadTooLarge)
		}
		return nil, Err(ErrInvalidBody).WithCause(err)
	}

	if len(bytes.TrimSpace(body)) == 0 {
		body = []byte("{}")
	}

	var present map[string]json.RawMessage
	if err := json.Unmarshal(body, &present); err != nil {
		return nil, Err(ErrInvalidBody).
			WithField("body", IssueInvalid, "Request body must be a JSON object.").
			WithCause(err)
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return nil, decodeError(err)
	}

	if err := Validate(dst); err != nil {
		return nil, err
	}

	fields := make(FieldSet, len(present))
	for key := range present {
		fields[key] = struct{}{}
	}
	return fields, nil
}

func decodeError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "body"
		}
		message := fmt.Sprintf("Expected a value of type %s.", typeErr.Type.String())
		if typeErr.Type == dateGoType {
			message = "Must be a date in YYYY-MM-DD format."
		}
		return Err(ErrValidationFailed).WithField(field, IssueInvalid, message)
	}

	if name, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		return Err(ErrUnknownFields).WithField(
			strings.Trim(name, `"`), IssueInvalid, "This field is not accepted by the endpoint.",
		)
	}

	return Err(ErrInvalidBody).WithCause(err)
}

// UUIDParam reads a URL path parameter and parses it as a UUID.
func UUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, Err(ErrInvalidID).
			WithField(name, IssueInvalid, "Must be a valid UUID.").
			WithCause(err)
	}
	return id, nil
}
