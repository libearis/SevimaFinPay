package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"ukt-payment-system/internal/middleware"
)

var ErrInvalidCredentials = errors.New("email atau password salah")

// Service handles user login verification and JWT token issuance.
type Service struct {
	repo      Repository
	jwtSecret string
	jwtTTL    time.Duration
}

// NewService constructs a new Auth Service.
func NewService(repo Repository, jwtSecret string, jwtTTL time.Duration) *Service {
	return &Service{
		repo:      repo,
		jwtSecret: jwtSecret,
		jwtTTL:    jwtTTL,
	}
}

// Login authenticates a user and signs a JWT containing their institusi_id and role.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.jwtTTL)

	claims := middleware.JWTClaims{
		UserID:        user.ID,
		Role:          user.Role,
		InstitusiID:   user.InstitusiID,
		KodeInstitusi: user.KodeInstitusi,
		Email:         user.Email,
		Name:          user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token:     signedToken,
		ExpiresAt: expiresAt,
		User:      *user,
	}, nil
}

// GetProfile retrieves the authenticated user's profile strictly scoped to their institusi_id.
func (s *Service) GetProfile(ctx context.Context, userID, institusiID uuid.UUID) (*UserWithInstitution, error) {
	return s.repo.GetByIDAndInstitusi(ctx, userID, institusiID)
}
