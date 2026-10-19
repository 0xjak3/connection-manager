package dto

import "time"

type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// UpdateUserRequest is a partial update: at least one field must be present.
type UpdateUserRequest struct {
	Email    *string `json:"email" validate:"required_without=Password,omitnil,email"`
	Password *string `json:"password" validate:"required_without=Email,omitnil,min=8,max=72"`
}

// UserResponse never carries the password or its hash.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
