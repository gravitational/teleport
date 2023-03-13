package gitlab

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIDTokenSource_GetIDToken(t *testing.T) {
	t.Run("value present", func(t *testing.T) {
		its := &IDTokenSource{
			getEnv: func(key string) string {
				if key == "TBOT_GITLAB_JWT" {
					return "foo"
				}
				return ""
			},
		}
		tok, err := its.GetIDToken()
		require.NoError(t, err)
		require.Equal(t, "foo", tok)
	})

	t.Run("value missing", func(t *testing.T) {
		its := &IDTokenSource{
			getEnv: func(key string) string {
				return ""
			},
		}
		tok, err := its.GetIDToken()
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
		require.Equal(t, "", tok)
	})
}
