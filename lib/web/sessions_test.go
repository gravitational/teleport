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

package web

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

func TestRemoteClientCache(t *testing.T) {
	t.Parallel()

	var openCount atomic.Int32
	cache := remoteClientCache{}

	sa1 := newMockRemoteSite("a")
	sa2 := newMockRemoteSite("a")
	sb := newMockRemoteSite("b")

	err1 := errors.New("c1")
	err2 := errors.New("c2")

	require.NoError(t, cache.addRemoteClient(sa1, newMockClientI(&openCount, err1)))
	require.Equal(t, int32(1), openCount.Load())

	require.ErrorIs(t, cache.addRemoteClient(sa2, newMockClientI(&openCount, nil)), err1)
	require.Equal(t, int32(1), openCount.Load())

	require.NoError(t, cache.addRemoteClient(sb, newMockClientI(&openCount, err2)))
	require.Equal(t, int32(2), openCount.Load())

	var aggrErr trace.Aggregate
	require.ErrorAs(t, cache.Close(), &aggrErr)
	require.ElementsMatch(t, []error{err2}, aggrErr.Errors())

	require.Zero(t, openCount.Load())
}

func newMockRemoteSite(name string) reversetunnelclient.RemoteSite {
	return &mockRemoteSite{name: name}
}

type mockRemoteSite struct {
	reversetunnelclient.RemoteSite
	name string
}

func (m *mockRemoteSite) GetName() string {
	return m.name
}

func newMockClientI(openCount *atomic.Int32, closeErr error) authclient.ClientI {
	openCount.Add(1)
	return &mockClientI{openCount: openCount, closeErr: closeErr}
}

type mockClientI struct {
	authclient.ClientI
	openCount *atomic.Int32
	closeErr  error
}

func (m *mockClientI) Close() error {
	m.openCount.Add(-1)
	return m.closeErr
}

func (m *mockClientI) GetDomainName(ctx context.Context) (string, error) {
	return "test", nil
}

func TestGetUserClient(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var openCount atomic.Int32
	sctx := SessionContext{
		cfg: SessionContextConfig{
			RootClusterName: "local",
			newRemoteClient: func(ctx context.Context, sessionContext *SessionContext, site reversetunnelclient.RemoteSite) (authclient.ClientI, error) {
				return newMockClientI(&openCount, nil), nil
			},
		},
	}

	localSite := &mockRemoteSite{name: "local"}
	remoteSite := &mockRemoteSite{name: "remote"}

	// getting a client for the local site should return
	// the RootClient from SessionContextConfig
	clt, err := sctx.GetUserClient(ctx, localSite)
	require.NoError(t, err)
	require.Nil(t, clt)
	require.Zero(t, openCount.Load())

	// getting a client a remote site for the first time
	// should call newRemoteClient from SessionContextConfig
	// and increment openCount
	clt, err = sctx.GetUserClient(ctx, remoteSite)
	require.NoError(t, err)
	require.NotNil(t, clt)
	require.Equal(t, int32(1), openCount.Load())

	// getting a client a remote site a second time
	// should return the cached client and not call
	// newRemoteClient from SessionContextConfig
	clt, err = sctx.GetUserClient(ctx, remoteSite)
	require.NoError(t, err)
	require.NotNil(t, clt)
	require.Equal(t, int32(1), openCount.Load())

	// clear the remote cache
	require.NoError(t, sctx.remoteClientCache.Close())
	require.Zero(t, openCount.Load())

	// now attempt to get the same remote site concurrently
	// and ensure that the first request creates the client
	// and the second request is provided the cached value
	type result struct {
		clt authclient.ClientI
		err error
	}

	resultCh := make(chan result, 2)
	go func() {
		clt, err := sctx.GetUserClient(ctx, remoteSite)
		resultCh <- result{clt: clt, err: err}
	}()

	go func() {
		clt, err := sctx.GetUserClient(ctx, remoteSite)
		resultCh <- result{clt: clt, err: err}
	}()

	timeout := time.After(10 * time.Second)
	clients := make([]authclient.ClientI, 2)
	for i := 0; i < 2; i++ {
		select {
		case res := <-resultCh:
			require.NoError(t, res.err)
			require.NotNil(t, res.clt)
			clients[i] = res.clt
		case <-timeout:
			t.Fatalf("Timed out waiting for user client results")
		}
	}

	// ensure that only one client was created and that
	// both clients returned are functional
	require.Equal(t, int32(1), openCount.Load())
	for i := 0; i < 2; i++ {
		domain, err := clients[i].GetDomainName(ctx)
		require.NoError(t, err)
		require.Equal(t, "test", domain)
	}
}
