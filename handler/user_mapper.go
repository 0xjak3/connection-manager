package handler

import (
	"connection_manager/domain"
	"connection_manager/dto"
)

func toUser(u domain.User) dto.User {
	return dto.User{
		ID:        u.ID,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func toUsers(users []domain.User) []dto.User {
	out := make([]dto.User, 0, len(users))
	for _, u := range users {
		out = append(out, toUser(u))
	}
	return out
}
