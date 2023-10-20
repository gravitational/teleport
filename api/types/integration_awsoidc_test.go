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

package types

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestGenerateAWSOIDCTokenRequest validates that the required fields are checked.
func TestGenerateAWSOIDCTokenRequest(t *testing.T) {
	t.Run("error when no issuer is provided", func(t *testing.T) {
		req := GenerateAWSOIDCTokenRequest{}

		err := req.CheckAndSetDefaults()
		require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %+v", err)
	})

	t.Run("success when issuer is provided", func(t *testing.T) {
		req := GenerateAWSOIDCTokenRequest{
			Issuer: "https://example.com",
		}

		err := req.CheckAndSetDefaults()
		require.NoError(t, err)
	})
}
