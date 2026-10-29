package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"ukt-payment-system/internal/middleware"
	"ukt-payment-system/pkg/response"
)

// Handler handles HTTP endpoints for authentication.
type Handler struct {
	service *Service
}

// NewHandler creates a new Auth HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Login handles POST /api/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAYLOAD", "Email dan password wajib diisi")
		return
	}

	res, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email atau password yang Anda masukkan salah")
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memproses autentikasi")
		return
	}

	response.OK(c, res)
}

// Me handles GET /api/me.
func (h *Handler) Me(c *gin.Context) {
	userID, ok1 := middleware.GetUserID(c)
	institusiID, ok2 := middleware.GetInstitusiID(c)
	if !ok1 || !ok2 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi pengguna tidak valid")
		return
	}

	profile, err := h.service.GetProfile(c.Request.Context(), userID, institusiID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "USER_NOT_FOUND", "Data profil pengguna tidak ditemukan")
		return
	}

	response.OK(c, profile)
}
