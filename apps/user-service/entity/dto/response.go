package dto

import "time"

// UserResponse is the read model returned to callers.
type UserResponse struct {
	ID        string    `json:"id"`
	CompanyID string    `json:"company_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// RegisterResponse wraps the created user.
type RegisterResponse struct {
	User      UserResponse `json:"user"`
	CompanyID string       `json:"company_id"`
}

// LoginResponse contains the issued token and basic user info.
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	User        UserResponse `json:"user"`
}
