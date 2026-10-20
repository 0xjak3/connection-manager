package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"connection_manager/domain"
	"connection_manager/dto"
)

// ErrorHandler is the only place that maps errors to HTTP status codes.
func ErrorHandler(c fiber.Ctx, err error) error {
	status, code, msg := classify(err)
	if status == fiber.StatusInternalServerError {
		slog.ErrorContext(c.Context(), "unhandled error", "method", c.Method(), "path", c.Path(), "err", err)
	}
	return c.Status(status).JSON(dto.ErrorResponse{Error: dto.ErrorBody{Code: code, Message: msg}})
}

func classify(err error) (status int, code dto.ErrorBodyCode, msg string) {
	var verrs validator.ValidationErrors
	var bindErr *fiber.BindError
	var fErr *fiber.Error
	switch {
	case errors.As(err, &verrs):
		return fiber.StatusBadRequest, dto.ErrorBodyCodeInvalidRequest, validationMessage(verrs)
	case errors.As(err, &bindErr):
		return fiber.StatusBadRequest, dto.ErrorBodyCodeInvalidRequest, "malformed request body"
	// Bind returns the fiber.ErrUnprocessableEntity sentinel itself for a
	// missing/unsupported Content-Type, and errors.Is matches it by pointer.
	// Handlers that need a 422 must use fiber.NewError(fiber.StatusUnprocessableEntity, ...)
	// (a new *fiber.Error, mapped to http_error below) and never return the
	// fiber.ErrUnprocessableEntity sentinel themselves, or it is reported as
	// a 400 invalid_request.
	case errors.Is(err, fiber.ErrUnprocessableEntity):
		return fiber.StatusBadRequest, dto.ErrorBodyCodeInvalidRequest, "unsupported or missing Content-Type"
	case errors.As(err, &fErr):
		return fErr.Code, dto.ErrorBodyCodeHTTPError, fErr.Message
	case errors.Is(err, domain.ErrValidation):
		return fiber.StatusBadRequest, dto.ErrorBodyCodeValidationError, err.Error()
	case errors.Is(err, domain.ErrNotFound):
		return fiber.StatusNotFound, dto.ErrorBodyCodeNotFound, err.Error()
	case errors.Is(err, domain.ErrConflict):
		return fiber.StatusConflict, dto.ErrorBodyCodeConflict, err.Error()
	case errors.Is(err, domain.ErrInvalidState):
		return fiber.StatusConflict, dto.ErrorBodyCodeInvalidState, err.Error()
	case errors.Is(err, domain.ErrForbidden):
		return fiber.StatusForbidden, dto.ErrorBodyCodeForbidden, err.Error()
	default:
		return fiber.StatusInternalServerError, dto.ErrorBodyCodeInternalError, "internal server error"
	}
}

func validationMessage(verrs validator.ValidationErrors) string {
	parts := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		if p := jsonParam(fe.Tag(), fe.Param()); p != "" {
			parts = append(parts, fmt.Sprintf("%s failed %s=%s", fe.Field(), fe.Tag(), p))
		} else {
			parts = append(parts, fmt.Sprintf("%s failed %s", fe.Field(), fe.Tag()))
		}
	}
	return strings.Join(parts, "; ")
}

// fieldParamTags are validator tags whose param names other struct fields,
// either every word (required_without=Email Password) or every other word
// starting with the first (required_if=Kind admin).
var fieldParamTags = map[string]int{
	"required_with": 1, "required_with_all": 1, "required_without": 1, "required_without_all": 1,
	"excluded_with": 1, "excluded_with_all": 1, "excluded_without": 1, "excluded_without_all": 1,
	"eqfield": 1, "nefield": 1, "gtfield": 1, "gtefield": 1, "ltfield": 1, "ltefield": 1,
	"fieldcontains": 1, "fieldexcludes": 1,
	"required_if": 2, "required_unless": 2, "excluded_if": 2, "excluded_unless": 2,
}

// jsonParam rewrites the Go field names in a tag param to JSON names, so
// messages never leak Go names (required_without=Password → password).
// Request JSON keys are snake_case (handler-dto.md), so the JSON name is the
// snake_case form of the Go name.
func jsonParam(tag, param string) string {
	step, ok := fieldParamTags[tag]
	if !ok || param == "" {
		return param
	}
	words := strings.Fields(param)
	for i := 0; i < len(words); i += step {
		words[i] = snakeCase(words[i])
	}
	return strings.Join(words, " ")
}

// snakeCase converts a Go identifier to snake_case: Password → password,
// CreatedAt → created_at, UserID → user_id, HTTPAddr → http_addr.
func snakeCase(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i, c := range r {
		if unicode.IsUpper(c) {
			prevLower := i > 0 && (unicode.IsLower(r[i-1]) || unicode.IsDigit(r[i-1]))
			acronymEnd := i > 0 && unicode.IsUpper(r[i-1]) && i+1 < len(r) && unicode.IsLower(r[i+1])
			if prevLower || acronymEnd {
				b.WriteByte('_')
			}
			c = unicode.ToLower(c)
		}
		b.WriteRune(c)
	}
	return b.String()
}
