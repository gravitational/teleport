// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func TestPinIdentitySameClusterFails(t *testing.T) {
	source := &PingResult{ClusterName: "cluster-a", ClusterID: "id-123"}
	target := &PingResult{ClusterName: "cluster-a", ClusterID: "id-123"}
	err := PinIdentity(source, target)
	require.ErrorContains(t, err, "same cluster")
}

func TestPinIdentityDifferentClusterSucceeds(t *testing.T) {
	source := &PingResult{ClusterName: "legacy-1", ClusterID: "id-111"}
	target := &PingResult{ClusterName: "scoped", ClusterID: "id-222"}
	err := PinIdentity(source, target)
	require.NoError(t, err)
}

func TestDetectIdentityFormatDirectory(t *testing.T) {
	dir := t.TempDir()
	fmt, err := DetectIdentityFormat(dir)
	require.NoError(t, err)
	require.Equal(t, FormatProfile, fmt)
}

func TestDetectIdentityFormatFile(t *testing.T) {
	tmp := t.TempDir() + "/identity"
	require.NoError(t, writeTestFile(tmp, "-----BEGIN"))
	fmt, err := DetectIdentityFormat(tmp)
	require.NoError(t, err)
	require.Equal(t, FormatIdentityFile, fmt)
}

func TestPingResultScopesEnabled(t *testing.T) {
	p := &PingResult{ScopesAuth: "enabled", ScopesProxy: true}
	require.True(t, p.ScopesEnabled())

	p2 := &PingResult{ScopesAuth: "disabled", ScopesProxy: true}
	require.False(t, p2.ScopesEnabled())

	p3 := &PingResult{ScopesAuth: "enabled", ScopesProxy: false}
	require.False(t, p3.ScopesEnabled())
}
