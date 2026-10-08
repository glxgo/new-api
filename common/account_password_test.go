package common

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestAccountPasswordFormats(t *testing.T) {
	t.Setenv("ACCOUNT_PASSWORD_HASH_ALGORITHM", "argon2id")
	password := "  密码 with spaces  "
	hash, err := HashAccountPassword(password)
	require.NoError(t, err)
	require.True(t, ValidatePasswordAndHash(password, hash))
	require.False(t, ValidatePasswordAndHash(strings.TrimSpace(password), hash))
	legacy, err := Password2Hash("legacy short")
	require.NoError(t, err)
	require.True(t, ValidatePasswordAndHash("legacy short", legacy))
	require.False(t, ValidatePasswordAndHash("wrong", hash))
	require.False(t, ValidatePasswordAndHash(password, strings.Replace(hash, "m=19456", "m=999999999", 1)))
	_, err = HashAccountPassword(strings.Repeat("界", 129))
	require.Error(t, err)
	_, err = HashAccountPassword(strings.Repeat("界", 128))
	require.NoError(t, err)
}
