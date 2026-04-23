package directory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test character sample set covers allowed characters for MS365
// https://learn.microsoft.com/en-us/troubleshoot/microsoft-365-apps/office-suite-issues/username-contains-special-character#cause
// which are not supported in Teleport.
// There are other characters such as forward slash (/)
// which MS365 itself does not support but the AD synced user
// accounts can contain them.
func TestIsUsernameValid(t *testing.T) {
	testCases := []struct {
		name         string
		username     string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:         "simple",
			username:     "alice@example.com",
			errAssertion: require.NoError,
		},
		{
			name:         "suported special characters",
			username:     "a_l:i.c-e+1@example.com",
			errAssertion: require.NoError,
		},
		{
			name:         "unsupported single quote",
			username:     "a'lice@example.com",
			errAssertion: require.Error,
		},
		{
			name:         "unsupported (/) separator",
			username:     `a/lice@example.com`,
			errAssertion: require.Error,
		},
		{
			name:         "unsupported (//) separator",
			username:     `a//lice@example.com`,
			errAssertion: require.Error,
		},
		{
			name:         `unsupported (\) separator`,
			username:     `a\lice@example.com`,
			errAssertion: require.Error,
		},
		{
			name:         `unsupported (\/) separator`,
			username:     `a\/lice@example.com`,
			errAssertion: require.Error,
		},
		{
			name:         "unsupported (#) character",
			username:     `a#lice@example.com`,
			errAssertion: require.Error,
		},
		{
			name:         "unsupported (!) character",
			username:     `a!lice@example.com`,
			errAssertion: require.Error,
		},
		{
			name:         "unsupported (^) character",
			username:     `a^lice@example.com`,
			errAssertion: require.Error,
		},
		{
			name:         "unsupported (~) character",
			username:     `a~lice@example.com`,
			errAssertion: require.Error,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.errAssertion(t, isValidUsername(tc.username))
		})
	}
}
