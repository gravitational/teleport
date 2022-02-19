// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/stretchr/testify/require"
)

// TestDesktopAccessDisabled makes sure desktop access can be disabled via modules.
// Since desktop connections require a cert, this is mediated via the cert generating function.
func TestDesktopAccessDisabled(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Desktop: false, // Explicily turn off desktop access.
		},
	})

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	r, err := p.a.GenerateWindowsDesktopCert(ctx, &proto.WindowsDesktopCertRequest{})
	require.Nil(t, r)
	require.Error(t, err)
	require.Contains(t, err.Error(), "this Teleport cluster is not licensed for desktop access, please contact the cluster administrator")
}
