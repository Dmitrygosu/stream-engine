package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dmitrygosu/stream-engine/internal/auth"
	"github.com/Dmitrygosu/stream-engine/internal/model/user"
)

var secret = []byte("test-secret-that-is-long-enough-32b")

const audience = "stream-engine"

func TestTokens_RoundTrip(t *testing.T) {
	tokens := auth.NewTokens(secret, audience, time.Minute)
	id := uuid.New()

	token, err := tokens.Issue(id, user.RoleCreator)
	require.NoError(t, err)
	principal, err := tokens.Verify(token)

	require.NoError(t, err)
	assert.Equal(t, auth.Principal{UserID: id, Role: user.RoleCreator}, principal)
}

func sign(t *testing.T, method jwt.SigningMethod, key any, mutate func(*jwt.MapClaims)) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  uuid.NewString(),
		"role": "CREATOR",
		"aud":  audience,
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	if mutate != nil {
		mutate(&claims)
	}
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return token
}

func TestTokens_RejectUntrustedTokens(t *testing.T) {
	cases := map[string]string{
		"garbage":        "not-a-jwt",
		"wrong secret":   sign(t, jwt.SigningMethodHS256, []byte("another-secret-of-thirty-two-b!!"), nil),
		"alg none":       sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, nil),
		"unexpected alg": sign(t, jwt.SigningMethodHS512, secret, nil),
		"expired": sign(t, jwt.SigningMethodHS256, secret, func(c *jwt.MapClaims) {
			(*c)["exp"] = time.Now().Add(-time.Hour).Unix()
		}),
		"no expiry":      sign(t, jwt.SigningMethodHS256, secret, func(c *jwt.MapClaims) { delete(*c, "exp") }),
		"other audience": sign(t, jwt.SigningMethodHS256, secret, func(c *jwt.MapClaims) { (*c)["aud"] = "billing" }),
		"unknown role":   sign(t, jwt.SigningMethodHS256, secret, func(c *jwt.MapClaims) { (*c)["role"] = "ROOT" }),
		"subject not id": sign(t, jwt.SigningMethodHS256, secret, func(c *jwt.MapClaims) { (*c)["sub"] = "alice" }),
	}
	tokens := auth.NewTokens(secret, audience, time.Minute)
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tokens.Verify(token)
			require.ErrorIs(t, err, auth.ErrInvalidToken)
		})
	}
}
