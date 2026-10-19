package handler

import (
	"connection_manager/domain"
	"connection_manager/dto"
)

func toUserResponse(u domain.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        u.ID.String(),
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func toUserResponses(users []domain.User) []dto.UserResponse {
	out := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toUserResponse(u))
	}
	return out
}
