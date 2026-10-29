package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestAuthRequiredAndRequireRole_HTTPMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-jwt-secret-key-minimum-32-chars"

	instID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	userID := uuid.MustParse("11111111-aaaa-4111-8111-000000000001")

	makeToken := func(role string, signingKey string) string {
		claims := JWTClaims{
			UserID:        userID,
			Role:          role,
			InstitusiID:   instID,
			KodeInstitusi: "UNIV-NUSANTARA",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, _ := tok.SignedString([]byte(signingKey))
		return s
	}

	router := gin.New()
	router.GET("/admin-only", AuthRequired(secret), RequireRole(RoleAdminKeuangan), func(c *gin.Context) {
		extractedInst, _ := GetInstitusiID(c)
		c.JSON(http.StatusOK, gin.H{"institusi_id": extractedInst.String()})
	})

	// 1. Tanpa Header Authorization -> 401 Unauthorized
	req1 := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("diharapkan 401 saat tanpa token, didapat %d", w1.Code)
	}

	// 2. Token dengan Role MAHASISWA mengakses endpoint ADMIN_KEUANGAN -> 403 Forbidden
	req2 := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req2.Header.Set("Authorization", "Bearer "+makeToken(RoleMahasiswa, secret))
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("diharapkan 403 saat mahasiswa mengakses endpoint admin, didapat %d", w2.Code)
	}

	// 3. Token dengan Role ADMIN_KEUANGAN yang sah -> 200 OK & institusi_id terekstrak dengan benar
	req3 := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req3.Header.Set("Authorization", "Bearer "+makeToken(RoleAdminKeuangan, secret))
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("diharapkan 200 OK untuk admin sah, didapat %d", w3.Code)
	}
}
