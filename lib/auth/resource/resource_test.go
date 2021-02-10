/*
Copyright 2021 Gravitational, Inc.

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

package resource

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"

	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

// TestOptions tests command options operations
func TestOptions(t *testing.T) {
	// test empty scenario
	out := AddOptions(nil)
	require.Empty(t, out)

	// make sure original option list is not affected
	in := []auth.MarshalOption{}
	out = AddOptions(in, WithResourceID(1))
	require.Len(t, out, 1)

	cfg, err := CollectOptions(out)
	require.NoError(t, err)
	require.Equal(t, cfg.ID, int64(1))

	// Add a couple of other parameters
	out = AddOptions(in, WithResourceID(2), SkipValidation(), WithVersion(types.V2))
	require.Len(t, out, 3)
	cfg, err = CollectOptions(out)
	require.NoError(t, err)
	require.Equal(t, cfg.ID, int64(2))
	require.True(t, cfg.SkipValidation)
	require.Equal(t, cfg.Version, types.V2)
}
