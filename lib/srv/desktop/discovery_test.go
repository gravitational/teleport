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

package desktop

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// TestDiscoveryLDAPFilter verifies that WindowsService produces a valid
// LDAP filter when given valid configuration.
func TestDiscoveryLDAPFilter(t *testing.T) {
	for _, test := range []struct {
		desc    string
		filters []string
		assert  require.ErrorAssertionFunc
	}{
		{
			desc:   "OK - no custom filters",
			assert: require.NoError,
		},
		{
			desc:    "OK - custom filters",
			filters: []string{"(computerName=test)", "(location=Oakland)"},
			assert:  require.NoError,
		},
		{
			desc:    "NOK - invalid custom filter",
			filters: []string{"invalid"},
			assert:  require.Error,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			s := new(WindowsService)
			filter := s.ldapSearchFilter(test.filters)
			_, err := ldap.CompileFilter(filter)
			test.assert(t, err)
		})
	}
}

func TestAppliesLDAPLabels(t *testing.T) {
	l := make(map[string]string)
	entry := ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
		attrDNSHostName:       {"foo.example.com"},
		attrName:              {"foo"},
		attrOS:                {"Windows Server"},
		attrOSVersion:         {"6.1"},
		attrDistinguishedName: {"CN=foo,OU=IT,DC=goteleport,DC=com"},
		attrCommonName:        {"foo"},
		"bar":                 {"baz"},
		"quux":                {""},
	})

	s := new(WindowsService)
	s.applyLabelsFromLDAP(entry, l, &servicecfg.LDAPDiscoveryConfig{
		BaseDN:          "*",
		LabelAttributes: []string{"bar"},
	})

	// check default labels
	require.Equal(t, types.OriginDynamic, l[types.OriginLabel])
	require.Equal(t, "foo.example.com", l[types.DiscoveryLabelWindowsDNSHostName])
	require.Equal(t, "foo", l[types.DiscoveryLabelWindowsComputerName])
	require.Equal(t, "Windows Server", l[types.DiscoveryLabelWindowsOS])
	require.Equal(t, "6.1", l[types.DiscoveryLabelWindowsOSVersion])

	// check OU label
	require.Equal(t, "OU=IT,DC=goteleport,DC=com", l[types.DiscoveryLabelWindowsOU])

	// check custom labels
	require.Equal(t, "baz", l["ldap/bar"])
	require.Empty(t, l["ldap/quux"])
}

func TestLabelsDomainControllers(t *testing.T) {
	s := &WindowsService{}
	for _, test := range []struct {
		desc   string
		entry  *ldap.Entry
		assert require.BoolAssertionFunc
	}{
		{
			desc: "DC",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				attrPrimaryGroupID: {writableDomainControllerGroupID},
			}),
			assert: require.True,
		},
		{
			desc: "RODC",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				attrPrimaryGroupID: {readOnlyDomainControllerGroupID},
			}),
			assert: require.True,
		},
		{
			desc: "computer",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				attrPrimaryGroupID: {"515"},
			}),
			assert: require.False,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			l := make(map[string]string)
			s.applyLabelsFromLDAP(test.entry, l, new(servicecfg.LDAPDiscoveryConfig))

			b, _ := strconv.ParseBool(l[types.DiscoveryLabelWindowsIsDomainController])
			test.assert(t, b)
		})
	}
}

// TestDNSErrors verifies that errors are handled quickly
// and do not block discovery for too long.
func TestDNSErrors(t *testing.T) {
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Logger:               slog.New(slog.DiscardHandler),
			Clock:                clockwork.NewRealClock(),
			ConnectedProxyGetter: reversetunnel.NewConnectedProxyGetter(),
		},
		dnsResolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return nil, errors.New("this resolver always fails")
			},
		},
	}

	// Attempting to resolve an empty hostname should return with an
	// error immediately and not wait for a network timeout.
	start := time.Now()
	_, err := s.lookupDesktop(t.Context(), "")
	require.Less(t, time.Since(start), dnsQueryTimeout-1*time.Second)
	require.Error(t, err)
}

func TestDynamicWindowsDiscovery(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
			ClusterName: "test",
			Dir:         t.TempDir(),
			AuditLog:    &eventstest.MockAuditLog{Emitter: new(eventstest.MockRecorderEmitter)},
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, authServer.Close()) })

		tlsServer, err := authServer.NewTestTLSServer(authtest.WithBufconnListener())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

		client, err := tlsServer.NewClient(authtest.TestServerID(types.RoleWindowsDesktop, "test-host-id"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, client.Close()) })

		for _, testCase := range []struct {
			name     string
			labels   map[string]string
			expected int
		}{
			{
				name:     "no labels",
				expected: 0,
			},
			{
				name:     "no matching labels",
				labels:   map[string]string{"xyz": "abc"},
				expected: 0,
			},
			{
				name:     "matching labels",
				labels:   map[string]string{"foo": "bar"},
				expected: 1,
			},
			{
				name:     "matching wildcard labels",
				labels:   map[string]string{"abc": "abc"},
				expected: 1,
			},
		} {
			// We can't use t.Run() as we're already in a synctest bubble.
			t.Logf("executing test case %v", testCase.name)
			testDynamicWindowsDiscovery(t, client, authServer.AuthServer, testCase.labels, testCase.expected)
		}
	})
}

func testDynamicWindowsDiscovery(t *testing.T, client *authclient.Client, auth *auth.Server, labels map[string]string, wantCount int) {
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Heartbeat: HeartbeatConfig{
				HostUUID: "1234",
			},
			Logger:               slog.New(slog.DiscardHandler),
			Clock:                clockwork.NewRealClock(),
			AuthClient:           client,
			AccessPoint:          client,
			ConnectedProxyGetter: reversetunnel.NewConnectedProxyGetter(),
			ResourceMatchers: []services.ResourceMatcher{
				{Labels: types.Labels{"foo": {"bar"}}},
				{Labels: types.Labels{"abc": {"*"}}},
			},
		},
	}

	// Clear all desktops to start the test case fresh.
	require.NoError(t, auth.DeleteAllWindowsDesktops(t.Context()))
	var key string
	for {
		page, next, err := auth.ListDynamicWindowsDesktops(t.Context(), 0, key)
		require.NoError(t, err)
		for _, dwd := range page {
			require.NoError(t, auth.DeleteDynamicWindowsDesktop(t.Context(), dwd.GetName()))
		}
		if next == "" {
			break
		}
		key = next
	}

	// Defer cancellation (instead of t.Cleanup) because this
	// function is called for multiple test cases in a single Go test.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	require.NoError(t, s.startDynamicReconciler(ctx))
	synctest.Wait()

	desktop, err := types.NewDynamicWindowsDesktopV1(
		"test", labels,
		types.DynamicWindowsDesktopSpecV1{Addr: "addr"},
	)
	require.NoError(t, err)

	dynamicWindowsClient := client.DynamicDesktopClient()
	_, err = dynamicWindowsClient.CreateDynamicWindowsDesktop(ctx, desktop)
	require.NoError(t, err)

	synctest.Wait()

	desktops, err := client.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.Len(t, desktops, wantCount)
	if wantCount > 0 {
		require.Equal(t, desktop.GetName(), desktops[0].GetName())
		require.Equal(t, desktop.GetAddr(), desktops[0].GetAddr())
	}

	desktop.Spec.Addr = "addr2"
	_, err = dynamicWindowsClient.UpsertDynamicWindowsDesktop(ctx, desktop)
	require.NoError(t, err)

	synctest.Wait()

	desktops, err = client.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.Len(t, desktops, wantCount)
	if wantCount > 0 {
		require.Equal(t, desktop.GetName(), desktops[0].GetName())
		require.Equal(t, desktop.GetAddr(), desktops[0].GetAddr())
	}

	require.NoError(t, dynamicWindowsClient.DeleteDynamicWindowsDesktop(ctx, "test"))

	synctest.Wait()

	desktops, err = client.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.Empty(t, desktops)
}

func TestDynamicWindowsDiscoveryExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
			ClusterName: "test",
			Dir:         t.TempDir(),
			AuditLog:    &eventstest.MockAuditLog{Emitter: new(eventstest.MockRecorderEmitter)},
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, authServer.Close()) })

		tlsServer, err := authServer.NewTestTLSServer(authtest.WithBufconnListener())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

		client, err := tlsServer.NewClient(authtest.TestServerID(types.RoleWindowsDesktop, "test-host-id"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, client.Close()) })

		dynamicWindowsClient := client.DynamicDesktopClient()

		s := &WindowsService{
			cfg: WindowsServiceConfig{
				Heartbeat: HeartbeatConfig{
					HostUUID: "1234",
				},
				Logger:      slog.New(slog.DiscardHandler),
				Clock:       clockwork.NewRealClock(),
				AuthClient:  client,
				AccessPoint: client,
				ResourceMatchers: []services.ResourceMatcher{
					{Labels: types.Labels{"foo": {"bar"}}},
				},
			},
		}

		require.NoError(t, s.startDynamicReconciler(t.Context()))

		synctest.Wait()

		desktop, err := types.NewDynamicWindowsDesktopV1(
			"test",
			map[string]string{"foo": "bar"},
			types.DynamicWindowsDesktopSpecV1{Addr: "addr"},
		)
		require.NoError(t, err)

		_, err = dynamicWindowsClient.CreateDynamicWindowsDesktop(t.Context(), desktop)
		require.NoError(t, err)

		synctest.Wait()

		desktops, err := client.GetWindowsDesktops(t.Context(), types.WindowsDesktopFilter{})
		require.NoError(t, err)
		require.Len(t, desktops, 1)
		require.Equal(t, "test", desktops[0].GetName())

		err = client.DeleteWindowsDesktop(t.Context(), s.cfg.Heartbeat.HostUUID, "test")
		require.NoError(t, err)

		desktops, err = client.GetWindowsDesktops(t.Context(), types.WindowsDesktopFilter{})
		require.NoError(t, err)
		require.Empty(t, desktops)

		synctest.Wait()
		time.Sleep(5 * time.Minute)
		synctest.Wait()

		desktops, err = client.GetWindowsDesktops(t.Context(), types.WindowsDesktopFilter{})
		require.NoError(t, err)
		require.Len(t, desktops, 1)
		require.Equal(t, "test", desktops[0].GetName())
	})
}

func TestCurrentDesktops(t *testing.T) {
	t.Parallel()
	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		ClusterName: "test",
		Dir:         t.TempDir(),
		AuditLog:    &eventstest.MockAuditLog{Emitter: new(eventstest.MockRecorderEmitter)},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	tlsServer, err := authServer.NewTestTLSServer(authtest.WithBufconnListener())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

	hostUUID := "test-host-id"
	client, err := tlsServer.NewClient(authtest.TestServerID(types.RoleWindowsDesktop, hostUUID))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Heartbeat: HeartbeatConfig{
				HostUUID: hostUUID,
			},
			Logger:      slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{})),
			Clock:       clockwork.NewFakeClock(),
			AccessPoint: client,
		},
	}

	desktops := []struct {
		name   string
		hostID string
		origin string
		expect bool
	}{
		{
			name:   "dynamic-same-host",
			hostID: hostUUID,
			origin: types.OriginDynamic,
			expect: true, // Should be included
		},
		{
			name:   "static-same-host",
			hostID: hostUUID,
			origin: types.OriginConfigFile,
			expect: false, // Wrong origin
		},
		{
			name:   "dynamic-different-host",
			hostID: "other-host",
			origin: types.OriginDynamic,
			expect: false, // Wrong host
		},
		{
			name:   "dynamic-same-host-2",
			hostID: hostUUID,
			origin: types.OriginDynamic,
			expect: true, // Should be included
		},
	}

	for _, d := range desktops {
		desktop, err := types.NewWindowsDesktopV3(d.name, map[string]string{
			types.OriginLabel: d.origin,
		}, types.WindowsDesktopSpecV3{
			Addr:   "addr-" + d.name,
			HostID: d.hostID,
		})
		require.NoError(t, err)
		err = tlsServer.Auth().UpsertWindowsDesktop(t.Context(), desktop)
		require.NoError(t, err)
	}

	t.Run("single page", func(t *testing.T) {
		// Call currentDesktops and verify results
		result := s.currentDesktops(t.Context())

		// Count expected desktops
		var expectedCount int
		for _, d := range desktops {
			if d.expect {
				expectedCount++
			}
		}

		require.Len(t, result, expectedCount)

		// Verify only the expected desktops are returned
		for _, d := range desktops {
			if d.expect {
				desktop, ok := result[d.name]
				require.True(t, ok, "expected desktop %s to be in results", d.name)
				require.Equal(t, d.name, desktop.GetName())
				require.Equal(t, d.hostID, desktop.GetHostID())
				originLabel, _ := desktop.GetLabel(types.OriginLabel)
				require.Equal(t, types.OriginDynamic, originLabel)
			} else {
				_, ok := result[d.name]
				require.False(t, ok, "desktop %s should not be in results", d.name)
			}
		}
	})

	t.Run("paginated", func(t *testing.T) {
		// forces a tiny page size so that  desktops span multiple pages
		// without needing thousands of entries.
		ap := &smallPageAccessPoint{WindowsDesktopAccessPoint: client, pageSize: 1}
		s.cfg.AccessPoint = ap

		result := s.currentDesktops(t.Context())
		require.Len(t, result, 2)
	})
}

// smallPageAccessPoint wraps a real access point and overrides ListWindowsDesktops
// to enforce a small page size, exercising multi-page iteration in tests.
type smallPageAccessPoint struct {
	authclient.WindowsDesktopAccessPoint
	pageSize int
}

func (a *smallPageAccessPoint) ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	req.Limit = a.pageSize
	return a.WindowsDesktopAccessPoint.ListWindowsDesktops(ctx, req)
}

func TestLDAPDiscoveryFailurePreservesDesktops(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
			ClusterName: "test",
			Dir:         t.TempDir(),
			AuditLog:    &eventstest.MockAuditLog{Emitter: new(eventstest.MockRecorderEmitter)},
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, authServer.Close()) })

		tlsServer, err := authServer.NewTestTLSServer(authtest.WithBufconnListener())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

		const hostUUID = "test-host-uuid"
		client, err := tlsServer.NewClient(authtest.TestServerID(types.RoleWindowsDesktop, hostUUID))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, client.Close()) })

		s := &WindowsService{
			closeCtx: t.Context(),
			cfg: WindowsServiceConfig{
				Heartbeat:         HeartbeatConfig{HostUUID: hostUUID},
				Logger:            slog.New(slog.DiscardHandler),
				Clock:             clockwork.NewRealClock(),
				AccessPoint:       client,
				AuthClient:        client,
				DiscoveryInterval: 5 * time.Minute,
			},

			// pre-cache a TLS config so we don't ask auth to issue a real cert
			// (there isn't a real LDAP server here to connect to)
			ldapTLSConfig:          new(tls.Config),
			ldapTLSConfigExpiresAt: time.Now().Add(24 * time.Hour),
		}

		originalExpiry := time.Now().Add(s.cfg.DiscoveryInterval * 3)

		// Populate last discovery results with some desktops
		lastDiscoveryResults := map[string]types.WindowsDesktop{}
		for i := 1; i <= 3; i++ {
			name := "desktop-" + strconv.Itoa(i)
			desktop, err := types.NewWindowsDesktopV3(name, map[string]string{
				types.OriginLabel:                      types.OriginDynamic,
				types.DiscoveryLabelWindowsDNSHostName: name + ".example.com",
				types.DiscoveryLabelWindowsOS:          "Windows Server 2019",
			}, types.WindowsDesktopSpecV3{
				HostID: hostUUID,
				Addr:   name + ".example.com:3389",
			})
			require.NoError(t, err)
			desktop.SetExpiry(originalExpiry)
			lastDiscoveryResults[name] = desktop

			require.NoError(t, tlsServer.Auth().UpsertWindowsDesktop(t.Context(), desktop))
		}
		s.lastDiscoveryResults = lastDiscoveryResults

		// Force the reconciler to run.
		// It will fail because we aren't running a real LDAP server in the test.
		require.NoError(t, s.startDesktopDiscovery())
		synctest.Wait()

		time.Sleep(15 * time.Second)
		preReconcile := time.Now()
		synctest.Wait()

		// Verify that the reconciler failure did not delete desktops and that
		// their expiry times were updated.
		for i := 1; i <= 3; i++ {
			name := "desktop-" + strconv.Itoa(i)
			desktops, err := client.GetWindowsDesktops(
				t.Context(),
				types.WindowsDesktopFilter{HostID: hostUUID, Name: name},
			)
			require.NoError(t, err)
			require.Len(t, desktops, 1)
			require.Equal(t, name, desktops[0].GetName())

			// Verify the TTL was updated
			actualExpiry := desktops[0].Expiry()
			require.Equal(t, 3*s.cfg.DiscoveryInterval, actualExpiry.Sub(preReconcile))
		}
	})
}
