package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

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

func classify(err error) (status int, code, msg string) {
	var verrs validator.ValidationErrors
	var bindErr *fiber.BindError
	var fErr *fiber.Error
	switch {
	case errors.As(err, &verrs):
		return fiber.StatusBadRequest, "invalid_request", validationMessage(verrs)
	case errors.As(err, &bindErr):
		return fiber.StatusBadRequest, "invalid_request", "malformed request body"
	case errors.As(err, &fErr):
		return fErr.Code, "http_error", fErr.Message
	case errors.Is(err, domain.ErrValidation):
		return fiber.StatusBadRequest, "validation_error", err.Error()
	case errors.Is(err, domain.ErrNotFound):
		return fiber.StatusNotFound, "not_found", err.Error()
	case errors.Is(err, domain.ErrConflict):
		return fiber.StatusConflict, "conflict", err.Error()
	case errors.Is(err, domain.ErrInvalidState):
		return fiber.StatusConflict, "invalid_state", err.Error()
	case errors.Is(err, domain.ErrForbidden):
		return fiber.StatusForbidden, "forbidden", err.Error()
	default:
		return fiber.StatusInternalServerError, "internal_error", "internal server error"
	}
}

func validationMessage(verrs validator.ValidationErrors) string {
	parts := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		if fe.Param() != "" {
			parts = append(parts, fmt.Sprintf("%s failed %s=%s", fe.Field(), fe.Tag(), fe.Param()))
		} else {
			parts = append(parts, fmt.Sprintf("%s failed %s", fe.Field(), fe.Tag()))
		}
	}
	return strings.Join(parts, "; ")
}
