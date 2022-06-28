// Copyright 2022 Gravitational, Inc
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

package web

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
)

func TestRemoteClientCache(t *testing.T) {
	t.Parallel()

	openCount := 0
	cache := remoteClientCache{}

	sa1 := newMockRemoteSite("a")
	sa2 := newMockRemoteSite("a")
	sb := newMockRemoteSite("b")

	err1 := errors.New("c1")
	err2 := errors.New("c2")

	require.NoError(t, cache.addRemoteClient(sa1, newMockClientI(&openCount, err1)))
	require.Equal(t, 1, openCount)

	require.ErrorIs(t, cache.addRemoteClient(sa2, newMockClientI(&openCount, nil)), err1)
	require.Equal(t, 1, openCount)

	require.NoError(t, cache.addRemoteClient(sb, newMockClientI(&openCount, err2)))
	require.Equal(t, 2, openCount)

	var aggrErr trace.Aggregate
	require.ErrorAs(t, cache.Close(), &aggrErr)
	require.ElementsMatch(t, []error{err2}, aggrErr.Errors())

	require.Zero(t, openCount)
}

func newMockRemoteSite(name string) reversetunnel.RemoteSite {
	return &mockRemoteSite{name: name}
}

type mockRemoteSite struct {
	reversetunnel.RemoteSite
	name string
}

func (m *mockRemoteSite) GetName() string {
	return m.name
}

func newMockClientI(openCount *int, closeErr error) auth.ClientI {
	*openCount++
	return &mockClientI{openCount: openCount, closeErr: closeErr}
}

type mockClientI struct {
	auth.ClientI
	openCount *int
	closeErr  error
}

func (m *mockClientI) Close() error {
	*m.openCount--
	return m.closeErr
}
