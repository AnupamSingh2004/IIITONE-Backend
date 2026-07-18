package auth

import (
	"testing"
	"time"

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
