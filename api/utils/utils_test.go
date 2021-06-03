/*
Copyright 2015-2019 Gravitational, Inc.

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

package utils_test

import (
	"testing"

	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func assertIsBadParameter(r require.TestingT, err error, i ...interface{}) {
	require.True(r, trace.IsBadParameter(err))
}

// TestVersions tests versions compatibility checking
func TestVersions(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc      string
		client    string
		minClient string
		assertErr require.ErrorAssertionFunc
	}{
		{desc: "client older than min version", client: "1.0.0", minClient: "1.1.0", assertErr: assertIsBadParameter},
		{desc: "client same as min version", client: "1.0.0", minClient: "1.0.0"},
		{desc: "client newer than min version", client: "1.1.0", minClient: "1.0.0"},
		{desc: "pre-releases clients are ok", client: "1.1.0-alpha.1", minClient: "1.0.0"},
		{desc: "older pre-releases are no ok", client: "1.1.0-alpha.1", minClient: "1.1.0", assertErr: assertIsBadParameter},
	}
	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			err := utils.CheckVersions(tt.client, tt.minClient)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
