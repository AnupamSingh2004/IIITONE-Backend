package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIssueAndVerifyToken(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()

	token, err := IssueToken(secret, userID, "student", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := VerifyToken(secret, token)
	require.NoError(t, err)
	require.Equal(t, userID, claims.UserID)
	require.Equal(t, "student", claims.Role)
}

func TestVerifyToken_Expired(t *testing.T) {
	secret := "test-secret"
	token, err := IssueToken(secret, uuid.New(), "student", -time.Hour)
	require.NoError(t, err)

	_, err = VerifyToken(secret, token)
	require.Error(t, err)
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	token, err := IssueToken("secret-a", uuid.New(), "student", time.Hour)
	require.NoError(t, err)

	_, err = VerifyToken("secret-b", token)
	require.Error(t, err)
}

func TestVerifyToken_MalformedToken(t *testing.T) {
	_, err := VerifyToken("test-secret", "not-a-jwt-at-all")
	require.Error(t, err)
}

func TestVerifyToken_RejectsNonHS256HMAC(t *testing.T) {
	secret := "test-secret"
	claims := Claims{
		UserID: uuid.New(),
		Role:   "student",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, err = VerifyToken(secret, signed)
	require.Error(t, err)
}
