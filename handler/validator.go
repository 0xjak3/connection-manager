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
	// trimmed_email: the value is an email once surrounding whitespace is
	// removed. The value itself is passed on unchanged; the domain normalizes it.
	err := v.RegisterValidation("trimmed_email", func(fl validator.FieldLevel) bool {
		return v.Var(strings.TrimSpace(fl.Field().String()), "email") == nil
	})
	if err != nil {
		panic(err) // only fails for an invalid tag name: a programming error
	}
	return &StructValidator{v: v}
}

func (s *StructValidator) Validate(out any) error { return s.v.Struct(out) }
