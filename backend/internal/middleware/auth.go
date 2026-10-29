package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"ukt-payment-system/pkg/response"
)

const (
	CtxKeyUserID        = "user_id"
	CtxKeyRole          = "role"
	CtxKeyInstitusiID   = "institusi_id"
	CtxKeyKodeInstitusi = "kode_institusi"
)

// JWTClaims represents custom claims embedded inside the signed JWT token.
type JWTClaims struct {
	UserID        uuid.UUID `json:"user_id"`
	Role          string    `json:"role"`
	InstitusiID   uuid.UUID `json:"institusi_id"`
	KodeInstitusi string    `json:"kode_institusi"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	jwt.RegisteredClaims
}

// AuthRequired verifies the Bearer JWT token and injects tenant-isolated user claims into Gin context.
// Crucially, institusi_id is ALWAYS taken from the verified JWT token, never from request body/query params.
func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Token autentikasi Bearer JWT diperlukan")
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims := &JWTClaims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid || claims.InstitusiID == uuid.Nil || claims.UserID == uuid.Nil {
			response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "Token JWT tidak valid atau telah kedaluwarsa")
			c.Abort()
			return
		}

		c.Set(CtxKeyUserID, claims.UserID)
		c.Set(CtxKeyRole, claims.Role)
		c.Set(CtxKeyInstitusiID, claims.InstitusiID)
		c.Set(CtxKeyKodeInstitusi, claims.KodeInstitusi)

		c.Next()
	}
}

// GetInstitusiID extracts the authenticated user's institusi_id from Gin context.
func GetInstitusiID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(CtxKeyInstitusiID)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// GetUserID extracts the authenticated user's user_id from Gin context.
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(CtxKeyUserID)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok && id != uuid.Nil
}
