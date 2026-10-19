package dto

import "time"

// trimmed_email is a custom tag registered in handler/validator.go: the value
// is a valid email once surrounding whitespace is removed.
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,trimmed_email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// UpdateUserRequest is a partial update: at least one field must be present.
type UpdateUserRequest struct {
	Email    *string `json:"email" validate:"required_without=Password,omitnil,trimmed_email"`
	Password *string `json:"password" validate:"required_without=Email,omitnil,min=8,max=72"`
}

// UserResponse never carries the password or its hash.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
