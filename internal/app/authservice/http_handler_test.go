package authservice_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Dmitrygosu/stream-engine/internal/app/authservice"
)

func TestAuthAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newService()
	r := gin.New()
	authservice.NewHandler(svc).RegisterRoutes(r)

	post := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	register := `{"email":"bob@example.com","password":"correct horse"}`
	assert.Equal(t, http.StatusCreated, post("/api/v1/auth/register", register).Code)
	assert.Equal(t, http.StatusConflict, post("/api/v1/auth/register", register).Code)

	tooLong := `{"email":"eve@example.com","password":"` + strings.Repeat("p", 73) + `"}`
	assert.Equal(t, http.StatusBadRequest, post("/api/v1/auth/register", tooLong).Code, "bcrypt limit")
	admin := `{"email":"eve@example.com","password":"correct horse","role":"ADMIN"}`
	assert.Equal(t, http.StatusBadRequest, post("/api/v1/auth/register", admin).Code)

	ok := post("/api/v1/auth/login", register)
	assert.Equal(t, http.StatusOK, ok.Code)
	assert.Contains(t, ok.Body.String(), `"token"`)

	for _, body := range []string{
		`{"email":"bob@example.com","password":"wrong horse"}`,
		`{"email":"nobody@example.com","password":"correct horse"}`,
	} {
		rec := post("/api/v1/auth/login", body)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid_credentials", "unknown emails and wrong passwords look the same")
	}
}
