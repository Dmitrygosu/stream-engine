package auth

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Dmitrygosu/stream-engine/internal/httpx"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
)

const principalKey = "stream-engine.principal"

// Verifier turns a bearer token into the caller it identifies.
type Verifier interface {
	Verify(token string) (Principal, error)
}

// RequireAuth rejects requests without a valid bearer token and stores the
// caller in the request context.
func RequireAuth(verifier Verifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			unauthorized(c, "missing bearer token")
			return
		}
		principal, err := verifier.Verify(token)
		if err != nil {
			unauthorized(c, "invalid or expired token")
			return
		}
		c.Set(principalKey, principal)
		c.Next()
	}
}

// RequireRole lets the request through only for the given roles. It must run
// after RequireAuth.
func RequireRole(roles ...user.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := PrincipalFrom(c)
		if !ok || !slices.Contains(roles, principal.Role) {
			httpx.Abort(c, http.StatusForbidden, "forbidden", "your role does not allow this action")
			return
		}
		c.Next()
	}
}

// PrincipalFrom returns the caller stored by RequireAuth.
func PrincipalFrom(c *gin.Context) (Principal, bool) {
	value, ok := c.Get(principalKey)
	if !ok {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	return principal, ok
}

func unauthorized(c *gin.Context, message string) {
	c.Header("WWW-Authenticate", `Bearer realm="stream-engine"`)
	httpx.Abort(c, http.StatusUnauthorized, "unauthorized", message)
}
