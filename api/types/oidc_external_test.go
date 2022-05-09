package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOIDCClaimsRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		src  OIDCClaims
	}{
		{
			name: "empty",
			src:  OIDCClaims{},
		},
		{
			name: "full",
			src: OIDCClaims(map[string]interface{}{
				"email_verified": true,
				"groups":         []interface{}{"everyone", "idp-admin", "idp-dev"},
				"email":          "superuser@example.com",
				"sub":            "00001234abcd",
				"exp":            1652091713.0,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.src.Size())
			count, err := tt.src.MarshalTo(buf)
			require.NoError(t, err)
			require.Equal(t, tt.src.Size(), count)

			dst := &OIDCClaims{}
			err = dst.Unmarshal(buf)
			require.NoError(t, err)
			require.Equal(t, &tt.src, dst)
		})
	}
}
