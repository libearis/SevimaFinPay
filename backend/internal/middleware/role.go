package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"ukt-payment-system/pkg/response"
)

const (
	RoleAdminKeuangan = "ADMIN_KEUANGAN"
	RoleMahasiswa     = "MAHASISWA"
)

// RequireRole enforces role-based access control (RBAC) for endpoints.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get(CtxKeyRole)
		if !exists {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "Akses ditolak: role tidak ditemukan")
			c.Abort()
			return
		}

		userRole, ok := roleVal.(string)
		if !ok {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "Akses ditolak: format role tidak valid")
			c.Abort()
			return
		}

		for _, allowed := range allowedRoles {
			if userRole == allowed {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, "FORBIDDEN", "Akses ditolak: Anda tidak memiliki hak akses untuk fitur ini")
		c.Abort()
	}
}
