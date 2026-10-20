package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"connection_manager/domain"
	"connection_manager/dto"
	"connection_manager/usecase"
)

type userUsecase interface {
	Create(ctx context.Context, in usecase.CreateUserInput) (domain.User, error)
	Get(ctx context.Context, id uuid.UUID) (domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	Update(ctx context.Context, in usecase.UpdateUserInput) (domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserHandler struct{ uc userUsecase }

func NewUserHandler(uc userUsecase) *UserHandler {
	return &UserHandler{uc: uc}
}

func (h *UserHandler) Create(c fiber.Ctx) error {
	var req dto.CreateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return err
	}
	user, err := h.uc.Create(c.Context(), usecase.CreateUserInput{Email: req.Email, Password: req.Password})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dto.UserResponse{Data: toUser(user)})
}

func (h *UserHandler) List(c fiber.Ctx) error {
	users, err := h.uc.List(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(dto.UserListResponse{Data: toUsers(users)})
}

func (h *UserHandler) Get(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	user, err := h.uc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(dto.UserResponse{Data: toUser(user)})
}

func (h *UserHandler) Update(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var req dto.UpdateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return err
	}
	user, err := h.uc.Update(c.Context(), usecase.UpdateUserInput{ID: id, Email: req.Email, Password: req.Password})
	if err != nil {
		return err
	}
	return c.JSON(dto.UserResponse{Data: toUser(user)})
}

func (h *UserHandler) Delete(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	if err := h.uc.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func parseID(c fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, fiber.NewError(fiber.StatusBadRequest, "invalid id")
	}
	return id, nil
}
