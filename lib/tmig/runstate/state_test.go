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

package runstate

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	require.NoError(t, err)

	s.SetHost("uuid-1", HostState{Hostname: "host-1", Verdict: "AUTO", Status: "SATISFIED"})
	require.NoError(t, s.Save())

	// Reload
	s2, err := New(dir)
	require.NoError(t, err)
	h, ok := s2.GetHost("uuid-1")
	require.True(t, ok)
	require.Equal(t, "host-1", h.Hostname)
	require.Equal(t, "AUTO", h.Verdict)
}

func TestStateResume(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	require.NoError(t, err)
	s.SetHost("uuid-1", HostState{Hostname: "h1", Verdict: "AUTO", Status: "SATISFIED"})
	s.SetHost("uuid-2", HostState{Hostname: "h2", Verdict: "", Status: "PENDING"})
	s.Save()

	s2, err := New(dir)
	require.NoError(t, err)
	pending := s2.PendingHosts()
	require.Len(t, pending, 1)
	require.Equal(t, "uuid-2", pending[0])
}

func TestStatePath(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	require.Equal(t, filepath.Join(dir, "runstate.json"), s.Path())
}
