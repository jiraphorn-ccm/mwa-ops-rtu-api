package httpx

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validateOnce sync.Once
	validate     *validator.Validate
)

// Validator returns the process-wide validator, configured to report fields by
// their JSON name so the `errors[].field` values match the request body.
func Validator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New(validator.WithRequiredStructEnabled())
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" || name == "" {
				return fld.Name
			}
			return name
		})
	})
	return validate
}

// Validate runs struct validation and converts the result into an E100_003
// AppError carrying one entry per failed rule.
func Validate(v any) error {
	if err := Validator().Struct(v); err != nil {
		var invalid *validator.InvalidValidationError
		if errors.As(err, &invalid) {
			return Err(ErrInvalidBody).WithCause(err)
		}
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			appErr := Err(ErrValidationFailed)
			for _, fe := range verrs {
				appErr.WithField(fieldPath(fe), issueFor(fe.Tag()), describe(fe))
			}
			return appErr
		}
		return Err(ErrValidationFailed).WithCause(err)
	}
	return nil
}

// fieldPath keeps nested paths readable, e.g. "readings[0].parameter_key".
func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if idx := strings.Index(ns, "."); idx >= 0 {
		ns = ns[idx+1:]
	}
	return ns
}

func issueFor(tag string) string {
	switch tag {
	case "required", "required_with", "required_without", "required_if":
		return IssueRequired
	case "max", "min", "gt", "gte", "lt", "lte", "len":
		return IssueOutOfRange
	case "excluded_with", "excluded_without":
		return IssueXOR
	default:
		return IssueInvalid
	}
}

func describe(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required."
	case "oneof":
		return fmt.Sprintf("Must be one of: %s.", strings.ReplaceAll(fe.Param(), " ", ", "))
	case "max":
		return fmt.Sprintf("Must not exceed %s.", fe.Param())
	case "min":
		return fmt.Sprintf("Must be at least %s.", fe.Param())
	case "gt":
		return fmt.Sprintf("Must be greater than %s.", fe.Param())
	case "gte":
		return fmt.Sprintf("Must be greater than or equal to %s.", fe.Param())
	case "lt":
		return fmt.Sprintf("Must be less than %s.", fe.Param())
	case "lte":
		return fmt.Sprintf("Must be less than or equal to %s.", fe.Param())
	case "uuid", "uuid4":
		return "Must be a valid UUID."
	case "email":
		return "Must be a valid email address."
	case "url":
		return "Must be a valid URL."
	default:
		return fmt.Sprintf("Failed the '%s' rule.", fe.Tag())
	}
}
