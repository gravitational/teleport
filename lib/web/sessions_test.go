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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/cert"
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

func newMockClientI(openCount *atomic.Int32, closeErr error) auth.ClientI {
	openCount.Add(1)
	return &mockClientI{openCount: openCount, closeErr: closeErr}
}

type mockClientI struct {
	auth.ClientI
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
			newRemoteClient: func(ctx context.Context, sessionContext *SessionContext, site reversetunnelclient.RemoteSite) (auth.ClientI, error) {
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
		clt auth.ClientI
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
	clients := make([]auth.ClientI, 2)
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

func TestSessionCache_watcher(t *testing.T) {
	// Can't t.Parallel because of modules.SetTestModules.

	// Requires Enterprise to work.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	webSuite := newWebSuite(t)
	authServer := webSuite.server.AuthServer.AuthServer
	authClient := webSuite.proxyClient
	clock := webSuite.clock

	// cancel is used to make sure the sessionCache stops cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processedC := make(chan struct{})
	sessionCache, err := newSessionCache(ctx, sessionCacheOptions{
		proxyClient: authClient,
		accessPoint: authClient,
		servers: []utils.NetAddr{
			// An addr is required but unused.
			{Addr: "localhost:12345", AddrNetwork: "tcp"}},
		clock:                               clock,
		sessionLingeringThreshold:           1 * time.Minute,
		sessionWatcherStartImmediately:      true,
		sessionWatcherEventProcessedChannel: processedC,
	})
	require.NoError(t, err, "newSessionCache() failed")
	defer sessionCache.Close()

	// Sanity check active sessions.
	require.Zero(t,
		sessionCache.ActiveSessions(),
		"ActiveSessions() count mismatch")

	// Create realistic keys and certificates, newSessionContextFromSession
	// requires it.
	creds, err := cert.GenerateSelfSignedCert(nil /* hostNames */, nil /* ipAddresses */)
	require.NoError(t, err, "GenerateSelfSignedCert() failed")

	// Create "fake" sessions with the same sessionID using newSession.
	sessionID := uuid.NewString()
	newSession := func(t *testing.T) types.WebSession {
		expires := clock.Now().Add(1 * time.Hour)
		session, err := types.NewWebSession(sessionID, types.KindWebSession, types.WebSessionSpecV2{
			User:               "llama", // fake
			Pub:                []byte(`ceci n'est pas an SSH certificate`),
			Priv:               creds.PrivateKey,
			TLSCert:            creds.Cert,
			BearerToken:        "12345678",
			BearerTokenExpires: expires,
			Expires:            expires,
			IdleTimeout:        types.Duration(1 * time.Hour),
		})
		require.NoError(t, err, "NewWebSession() failed")
		return session
	}

	// Record session in cache.
	_, err = sessionCache.newSessionContextFromSession(ctx, newSession(t))
	require.NoError(t, err, "newSessionContextFromSession() failed")

	// Sanity check active sessions.
	require.Equal(t,
		1,
		sessionCache.ActiveSessions(),
		"ActiveSessions() count mismatch")

	updateSessionAndAssert := func(t *testing.T, hasDeviceExtensions bool, wantActiveSessions int) {
		t.Helper()

		// Update the WebSession.
		// Certs here don't need to be realistic, they are never parsed.
		sessionV2 := newSession(t).(*types.WebSessionV2)
		sessionV2.Spec.Pub = []byte(`new SSH certificate`)
		sessionV2.Spec.TLSCert = []byte(`new X.509 certificate`)
		sessionV2.Spec.HasDeviceExtensions = hasDeviceExtensions
		require.NoError(t,
			authServer.WebSessions().Upsert(ctx, sessionV2),
			"WebSessions.Upsert() failed",
		)

		timer := time.NewTimer(20 * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			t.Fatal("sessionCache didn't process an event before timeout")
		case <-processedC:
			assert.Equal(t, wantActiveSessions, sessionCache.ActiveSessions(), "sessionCache.ActiveSessions() mismatch")
		}
	}

	t.Run("non-device-extensions update doesn't evict session", func(t *testing.T) {
		updateSessionAndAssert(t, false /* hasDeviceExtensions */, 1 /* wantActiveSessions */)
	})

	t.Run("device extensions update evicts session", func(t *testing.T) {
		updateSessionAndAssert(t, true /* hasDeviceExtensions */, 0 /* wantActiveSessions */)
	})

	t.Run("session with device extensions not evicted", func(t *testing.T) {
		sessionV2 := newSession(t).(*types.WebSessionV2)
		sessionV2.Spec.HasDeviceExtensions = true

		// Record session in cache.
		_, err = sessionCache.newSessionContextFromSession(ctx, sessionV2)
		require.NoError(t, err, "newSessionContextFromSession() failed")
		// Sanity check.
		require.Equal(t, 1, sessionCache.ActiveSessions(), "ActiveSessions() count mismatch")

		updateSessionAndAssert(t, true /* hasDeviceExtensions */, 1 /* wantActiveSessions */)
	})
}
