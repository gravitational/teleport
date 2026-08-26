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

package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseResourcePath(f *testing.F) {
	f.Add("")
	f.Add("/api/v1/")
	f.Add("/apis/group/version")
	f.Add("/watch/abc/def/hij")
	f.Add("/apis/apps/v1")
	f.Add("/api/v1/pods")
	f.Add("/api/v1/clusterroles/name")
	f.Add("/api/v1/namespaces/namespace/pods")
	f.Add("/apis/apiregistration.k8s.io/v1/apiservices/name/status")
	f.Add("/api/v1/namespaces/{namespace}/pods/name/exec")
	f.Add("/api/v1/nodes/name/proxy/path")

	f.Fuzz(func(t *testing.T, path string) {
		require.NotPanics(t, func() {
			parseResourcePath(path)
		})
	})
}
