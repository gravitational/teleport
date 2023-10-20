/*
Copyright 2022 Gravitational, Inc.

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
