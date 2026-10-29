package auth

import (
	"time"

	"github.com/google/uuid"
)

// UserWithInstitution holds user account details joined with their bound institution.
type UserWithInstitution struct {
	ID            uuid.UUID `json:"id"`
	InstitusiID   uuid.UUID `json:"institusi_id"`
	KodeInstitusi string    `json:"kode_institusi"`
	NamaInstitusi string    `json:"nama_institusi"`
	NimNip        string    `json:"nim_nip"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
	Role          string    `json:"role"`
	CreatedAt     time.Time `json:"created_at"`
}

// LoginRequest represents the payload for POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse returns the JWT token and authenticated user profile.
type LoginResponse struct {
	Token     string              `json:"token"`
	ExpiresAt time.Time           `json:"expires_at"`
	User      UserWithInstitution `json:"user"`
}
