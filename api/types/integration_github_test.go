/*
Copyright 2024 Gravitational, Inc.

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

	"github.com/stretchr/testify/require"
)

func TestValidateGitHubOrganizationName(t *testing.T) {
	tests := []struct {
		name       string
		checkError require.ErrorAssertionFunc
	}{
		{
			name:       "valid-org",
			checkError: require.NoError,
		},
		{
			name:       "a",
			checkError: require.NoError,
		},
		{
			name:       "1-valid-start-with-digit",
			checkError: require.NoError,
		},
		{
			name:       "-invalid-start-with-hyphen",
			checkError: require.Error,
		},
		{
			name:       "invalid-end-with-hyphen-",
			checkError: require.Error,
		},
		{
			name:       "invalid charactersc",
			checkError: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checkError(t, ValidateGitHubOrganizationName(test.name))
		})
	}
}
