// Copyright 2025 Gravitational, Inc.
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

package capability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLookupKnownVersion(t *testing.T) {
	cap, ok := Lookup("17.3.1")
	require.True(t, ok)
	require.True(t, cap.Roles["Node"])
	require.True(t, cap.Roles["Kube"])
	require.False(t, cap.Roles["App"])
	require.False(t, cap.Roles["Db"])
	require.Contains(t, cap.Methods, "iam")
	require.Contains(t, cap.Methods, "bound_keypair")
}

func TestLookupUnknownVersion(t *testing.T) {
	_, ok := Lookup("99.0.0")
	require.False(t, ok)
}

func TestNeedsDriftProbe(t *testing.T) {
	require.False(t, NeedsDriftProbe("17.3.1"))
	require.True(t, NeedsDriftProbe("99.0.0"))
}
