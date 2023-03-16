/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
