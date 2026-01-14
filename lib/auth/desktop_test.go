/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

// TestDesktopAccessDisabled makes sure desktop access can be disabled via modules.
// Since desktop connections require a cert, this is mediated via the cert generating function.
func TestDesktopAccessDisabled(t *testing.T) {
	ctx := t.Context()
	p, err := newTestPack(ctx, testPackOptions{
		DataDir: t.TempDir(),
		Modules: modulestest.OSSModules(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if p.bk != nil {
			p.bk.Close()
		}
	})

	r, err := p.a.GenerateWindowsDesktopCert(ctx, &proto.WindowsDesktopCertRequest{})
	require.Nil(t, r)
	require.Error(t, err)
	require.Contains(t, err.Error(), "this Teleport cluster is not licensed for desktop access, please contact the cluster administrator")
}
