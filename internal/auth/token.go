// Package auth issues and verifies the access tokens of the API and provides
// the Gin middleware that enforces them.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Dmitrygosu/stream-engine/internal/model/user"
)

// ErrInvalidToken is returned for any token that cannot be trusted.
var ErrInvalidToken = errors.New("invalid token")

// clockSkew tolerates small clock differences between services.
const clockSkew = 30 * time.Second

// Principal is the authenticated caller of a request.
type Principal struct {
	UserID uuid.UUID
	Role   user.Role
}

// IsAdmin reports whether the caller may act on other users' resources.
func (p Principal) IsAdmin() bool { return p.Role == user.RoleAdmin }

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Tokens signs and verifies HS256 access tokens.
type Tokens struct {
	secret   []byte
	audience string
	ttl      time.Duration
}

func NewTokens(secret []byte, audience string, ttl time.Duration) *Tokens {
	return &Tokens{secret: secret, audience: audience, ttl: ttl}
}

// Issue signs a token for the user.
func (t *Tokens) Issue(userID uuid.UUID, role user.Role) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{t.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
		},
	})
	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, nil
}

// Verify checks the signature, algorithm, expiry and audience of a token and
// returns the caller it identifies.
func (t *Tokens) Verify(token string) (Principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		// Pinning the algorithm rules out "alg: none" and algorithm confusion.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithAudience(t.audience),
		jwt.WithLeeway(clockSkew),
	)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}

	userID, err := uuid.Parse(c.Subject)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	role := user.Role(c.Role)
	if !role.IsValid() {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: userID, Role: role}, nil
}
