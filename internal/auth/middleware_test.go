package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dmitrygosu/stream-engine/internal/auth"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
)

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := auth.NewTokens(secret, audience, time.Minute)

	r := gin.New()
	r.GET("/creators-only", auth.RequireAuth(tokens), auth.RequireRole(user.RoleCreator, user.RoleAdmin),
		func(c *gin.Context) {
			p, ok := auth.PrincipalFrom(c)
			require.True(t, ok)
			c.String(http.StatusOK, p.UserID.String())
		})

	tokenFor := func(role user.Role) string {
		token, err := tokens.Issue(uuid.New(), role)
		require.NoError(t, err)
		return token
	}
	cases := map[string]struct {
		header string
		want   int
	}{
		"no header":     {"", http.StatusUnauthorized},
		"not bearer":    {"Basic dXNlcjpwYXNz", http.StatusUnauthorized},
		"invalid token": {"Bearer nope", http.StatusUnauthorized},
		"viewer":        {"Bearer " + tokenFor(user.RoleViewer), http.StatusForbidden},
		"creator":       {"Bearer " + tokenFor(user.RoleCreator), http.StatusOK},
		"admin":         {"Bearer " + tokenFor(user.RoleAdmin), http.StatusOK},
	}
	for name, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/creators-only", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, tc.want, rec.Code, name)
		if tc.want == http.StatusUnauthorized {
			assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "Bearer", name)
		}
	}
}
