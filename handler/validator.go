package handler

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// StructValidator adapts go-playground/validator to fiber.StructValidator.
type StructValidator struct{ v *validator.Validate }

func NewStructValidator() *StructValidator {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Report JSON field names (e.g. "email") instead of Go field names in errors.
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
	return &StructValidator{v: v}
}

func (s *StructValidator) Validate(out any) error { return s.v.Struct(out) }
