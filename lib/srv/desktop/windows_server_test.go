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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base32"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/winpki"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func TestConfigWildcardBaseDN(t *testing.T) {
	cfg := &WindowsServiceConfig{
		Discovery: []servicecfg.LDAPDiscoveryConfig{
			{
				BaseDN: "*",
			},
		},
		LDAPConfig: servicecfg.LDAPConfig{
			Domain: "test.goteleport.com",
		},
	}
	require.NoError(t, cfg.checkAndSetDiscoveryDefaults())
	require.Equal(t, "DC=test,DC=goteleport,DC=com", cfg.Discovery[0].BaseDN)
}

func TestConfigDesktopDiscovery(t *testing.T) {
	for _, test := range []struct {
		desc    string
		baseDN  string
		filters []string
		assert  require.ErrorAssertionFunc
	}{
		{
			desc:   "NOK - invalid base DN",
			baseDN: "example.com",
			assert: require.Error,
		},
		{
			desc:    "NOK - invalid filter",
			baseDN:  "DC=example,DC=goteleport,DC=com",
			filters: []string{"invalid!"},
			assert:  require.Error,
		},
		{
			desc:   "OK - wildcard base DN",
			baseDN: "*",
			assert: require.NoError,
		},
		{
			desc:   "OK - no filters",
			baseDN: "DC=example,DC=goteleport,DC=com",
			assert: require.NoError,
		},
		{
			desc:    "OK - valid filters",
			baseDN:  "DC=example,DC=goteleport,DC=com",
			filters: []string{"(!(primaryGroupID=516))"},
			assert:  require.NoError,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			cfg := &WindowsServiceConfig{
				Discovery: []servicecfg.LDAPDiscoveryConfig{
					{
						BaseDN:  test.baseDN,
						Filters: test.filters,
					},
				},
			}
			test.assert(t, cfg.checkAndSetDiscoveryDefaults())
		})
	}
}

// TestGenerateCredentials verifies that the smartcard certificates generated
// by Teleport meet the requirements for Windows logon.
func TestGenerateCredentials(t *testing.T) {
	t.Parallel()

	const (
		clusterName = "test"
		user        = "test-user"
		domain      = "test.example.com"
	)

	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		ClusterName: clusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authServer.Close())
	})

	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, tlsServer.Close())
	})

	windowsCA := fetchDesktopCAInfo(t, authServer.AuthServer, types.WindowsCA)
	userCA := fetchDesktopCAInfo(t, authServer.AuthServer, types.UserCA)
	// Sanity check.
	require.NotEqual(t,
		windowsCA.SerialNumber, userCA.SerialNumber,
		"CA serial numbers must not match",
	)

	client, err := tlsServer.NewClient(authtest.TestServerID(types.RoleWindowsDesktop, "test-host-id"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	const testSID = "S-1-5-21-1329593140-2634913955-1900852804-500"

	for _, test := range []struct {
		name                    string
		activeDirectorySID      string
		wantSerialNumber        string
		wantCRLCommonName       string
		disableWindowsCASupport bool
	}{
		{
			name:               "no ad sid",
			activeDirectorySID: "",
			wantSerialNumber:   windowsCA.SerialNumber,
			wantCRLCommonName:  windowsCA.CRLCommonName,
		},
		{
			name:               "with ad sid",
			activeDirectorySID: testSID,
			wantSerialNumber:   windowsCA.SerialNumber,
			wantCRLCommonName:  windowsCA.CRLCommonName,
		},
		{
			name:                    "old agent without AD SID",
			activeDirectorySID:      "",
			wantSerialNumber:        userCA.SerialNumber,
			wantCRLCommonName:       userCA.CRLCommonName,
			disableWindowsCASupport: true,
		},
		{
			name:                    "old agent with AD SID",
			activeDirectorySID:      testSID,
			wantSerialNumber:        userCA.SerialNumber,
			wantCRLCommonName:       userCA.CRLCommonName,
			disableWindowsCASupport: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			certb, keyb, err := winpki.GenerateWindowsDesktopCredentials(ctx, client, &winpki.GenerateCredentialsRequest{
				Username:                          user,
				Domain:                            domain,
				TTL:                               5 * time.Minute,
				ClusterName:                       clusterName,
				ActiveDirectorySID:                test.activeDirectorySID,
				DisableWindowsCASupportForTesting: test.disableWindowsCASupport,
			})
			require.NoError(t, err)
			require.NotNil(t, certb)
			require.NotNil(t, keyb)

			cert, err := x509.ParseCertificate(certb)
			require.NoError(t, err)
			require.NotNil(t, cert)

			require.Equal(t, test.wantSerialNumber, cert.Issuer.SerialNumber, "Issuer.SerialNumber")
			require.Equal(t, user, cert.Subject.CommonName, "Subject.CommonName")
			require.Contains(t,
				cert.CRLDistributionPoints,
				`ldap:///CN=`+test.wantCRLCommonName+`,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=example,DC=com?certificateRevocationList?base?objectClass=cRLDistributionPoint`,
				"CRLDistributionPoints",
			)

			foundKeyUsage := false
			foundAltName := false
			foundAdUserMapping := false
			for _, extension := range cert.Extensions {
				switch {
				case extension.Id.Equal(winpki.EnhancedKeyUsageExtensionOID):
					foundKeyUsage = true
					var oids []asn1.ObjectIdentifier
					_, err = asn1.Unmarshal(extension.Value, &oids)
					require.NoError(t, err)
					require.Len(t, oids, 2)
					require.Contains(t, oids, winpki.ClientAuthenticationOID)
					require.Contains(t, oids, winpki.SmartcardLogonOID)
				case extension.Id.Equal(winpki.SubjectAltNameExtensionOID):
					foundAltName = true
					var san winpki.SubjectAltName[winpki.UPN]
					_, err = asn1.Unmarshal(extension.Value, &san)
					require.NoError(t, err)
					require.Equal(t, winpki.UPNOtherNameOID, san.OtherName.OID)
					require.Equal(t, user+"@"+domain, san.OtherName.Value.Value)
				case extension.Id.Equal(winpki.ADUserMappingExtensionOID):
					foundAdUserMapping = true
					var adUserMapping winpki.SubjectAltName[winpki.ADSid]
					_, err = asn1.Unmarshal(extension.Value, &adUserMapping)
					require.NoError(t, err)
					require.Equal(t, winpki.ADUserMappingInternalOID, adUserMapping.OtherName.OID)
					require.Equal(t, []byte(testSID), adUserMapping.OtherName.Value.Value)

				}
			}
			require.True(t, foundKeyUsage)
			require.True(t, foundAltName)
			require.Equal(t, test.activeDirectorySID != "", foundAdUserMapping)
		})
	}
}

type certAuthorityGetter interface {
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)
}

type desktopCAInfo struct {
	SerialNumber  string
	CRLCommonName string
}

func fetchDesktopCAInfo(
	t *testing.T,
	authClient certAuthorityGetter,
	caType types.CertAuthType,
) *desktopCAInfo {
	t.Helper()

	const loadKeys = false
	cas, err := authClient.GetCertAuthorities(t.Context(), caType, loadKeys)
	require.NoError(t, err)
	require.Len(t, cas, 1)
	ca := cas[0]

	keys := ca.GetActiveKeys()
	require.Len(t, keys.TLS, 1)

	cert, err := tlsca.ParseCertificatePEM(keys.TLS[0].Cert)
	require.NoError(t, err)

	return &desktopCAInfo{
		SerialNumber:  cert.SerialNumber.String(),
		CRLCommonName: base32.HexEncoding.EncodeToString(cert.SubjectKeyId) + "_" + ca.GetClusterName(),
	}
}

func TestEmitsRecordingEventsOnSend(t *testing.T) {
	clock := clockwork.NewFakeClock()
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Clock: clock,
		},
	}
	emitter := &eventstest.MockRecorderEmitter{}
	emitterPreparer := libevents.WithNoOpPreparer(emitter)

	delay := func() int64 { return 0 }
	handler := s.makeTDPSendHandler(context.Background(), emitterPreparer, delay, nil /* conn */, nil /* auditor */)

	msg := &tdpb.PNGFrame{Data: []byte{0x01, 0x02}}
	encoded, err := msg.Encode()
	require.NoError(t, err)
	handler(msg, encoded)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	dr, ok := e.(*events.DesktopRecording)
	require.True(t, ok)
	require.Equal(t, encoded, dr.TDPBMessage)
}

func TestSkipsExtremelyLargePNGs(t *testing.T) {
	clock := clockwork.NewFakeClock()
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Clock:  clock,
			Logger: slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{})),
		},
	}
	emitter := &eventstest.MockRecorderEmitter{}
	emitterPreparer := libevents.WithNoOpPreparer(emitter)

	// a fake PNG Frame message, which is way too big to be legitimate
	maliciousPNG := make([]byte, libevents.MaxProtoMessageSizeBytes+1)
	rand.Read(maliciousPNG)
	png := &tdpb.PNGFrame{Data: maliciousPNG}
	encoded, err := png.Encode()
	require.NoError(t, err)

	delay := func() int64 { return 0 }
	handler := s.makeTDPSendHandler(context.Background(), emitterPreparer, delay, nil /* conn */, nil /* auditor */)

	handler(png, encoded)

	require.Nil(t, emitter.LastEvent())
}

func TestEmitsRecordingEventsOnReceive(t *testing.T) {
	clock := clockwork.NewFakeClock()
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Clock: clock,
		},
	}
	emitter := &eventstest.MockRecorderEmitter{}
	emitterPreparer := libevents.WithNoOpPreparer(emitter)

	delay := func() int64 { return 0 }
	handler := s.makeTDPReceiveHandler(context.Background(), emitterPreparer, delay, nil /* conn */, nil /* auditor */)

	msg := &tdpb.MouseButton{
		Button:  tdpbv1.MouseButtonType_MOUSE_BUTTON_TYPE_LEFT,
		Pressed: true,
	}
	handler(msg)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	dr, ok := e.(*events.DesktopRecording)
	require.True(t, ok)
	decoded, err := tdpb.DecodePermissive(bytes.NewBuffer(dr.TDPBMessage))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff((*tdpbv1.MouseButton)(msg), (*tdpbv1.MouseButton)(decoded.(*tdpb.MouseButton)), protocmp.Transform()))
}

func TestEmitsClipboardSendEvents(t *testing.T) {
	_, audit := setup(testDesktop)
	emitter := &eventstest.MockRecorderEmitter{}
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Clock:   audit.clock,
			Emitter: emitter,
		},
	}

	handler := s.makeTDPReceiveHandler(
		context.Background(),
		libevents.WithNoOpPreparer(&libevents.DiscardRecorder{}),
		func() int64 { return 0 },
		&tdp.Conn{},
		audit,
	)

	fakeClipboardData := make([]byte, 1024)
	rand.Read(fakeClipboardData)

	start := s.cfg.Clock.Now().UTC()
	msg := &tdpb.ClipboardData{
		Data: fakeClipboardData,
	}
	handler(msg)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	cs, ok := e.(*events.DesktopClipboardSend)
	require.True(t, ok)
	require.Equal(t, int32(len(fakeClipboardData)), cs.Length)
	require.Equal(t, audit.sessionID, cs.SessionID)
	require.Equal(t, audit.desktop.GetAddr(), cs.DesktopAddr)
	require.Equal(t, audit.clusterName, cs.ClusterName)
	require.Equal(t, start, cs.Time)
}

func TestEmitsClipboardReceiveEvents(t *testing.T) {
	_, audit := setup(testDesktop)
	emitter := &eventstest.MockRecorderEmitter{}
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Clock:   audit.clock,
			Emitter: emitter,
		},
	}

	handler := s.makeTDPSendHandler(
		context.Background(),
		libevents.WithNoOpPreparer(&libevents.DiscardRecorder{}),
		func() int64 { return 0 },
		&tdp.Conn{},
		audit,
	)

	fakeClipboardData := make([]byte, 512)
	rand.Read(fakeClipboardData)

	start := s.cfg.Clock.Now().UTC()
	msg := &tdpb.ClipboardData{Data: fakeClipboardData}
	encoded, err := msg.Encode()
	require.NoError(t, err)
	handler(msg, encoded)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	cs, ok := e.(*events.DesktopClipboardReceive)
	require.True(t, ok, "expected DesktopClipboardReceive, got %T", e)
	require.Equal(t, int32(len(fakeClipboardData)), cs.Length)
	require.Equal(t, audit.sessionID, cs.SessionID)
	require.Equal(t, audit.desktop.GetAddr(), cs.DesktopAddr)
	require.Equal(t, audit.clusterName, cs.ClusterName)
	require.Equal(t, start, cs.Time)
}

func TestLoadTLSConfigForLDAP(t *testing.T) {
	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		ClusterName: "test-cluster",
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authServer.Close())
	})

	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, tlsServer.Close())
	})

	client, err := tlsServer.NewClient(authtest.TestServerID(types.RoleWindowsDesktop, "test-host-id"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	newWindowsService := func(clock clockwork.Clock, client *authclient.Client) *WindowsService {
		return &WindowsService{
			cfg: WindowsServiceConfig{
				Clock:      clock,
				AuthClient: client,
				Logger:     slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{})),
				LDAPConfig: servicecfg.LDAPConfig{
					Domain:   "test.example.com",
					Username: "test-user",
					Addr:     "ldap.example.com:389",
				},
			},
			closeCtx: context.Background(),
		}
	}

	t.Run("returns cached config when not expired", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		s := newWindowsService(clock, nil)

		expectedConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		s.ldapTLSConfig = expectedConfig
		s.ldapTLSConfigExpiresAt = clock.Now().Add(1 * time.Hour)

		config, err := s.loadTLSConfigForLDAP()
		require.NoError(t, err)
		require.Equal(t, expectedConfig, config)
	})

	t.Run("generates new config when cache is expired", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		s := newWindowsService(clock, client)

		oldConfig := &tls.Config{MinVersion: tls.VersionTLS10}
		s.ldapTLSConfig = oldConfig
		s.ldapTLSConfigExpiresAt = clock.Now().Add(-1 * time.Hour)

		config, err := s.loadTLSConfigForLDAP()
		require.NoError(t, err)
		require.NotNil(t, config)
		require.NotEqual(t, oldConfig, config)

		require.Equal(t, config, s.ldapTLSConfig)
		require.True(t, s.ldapTLSConfigExpiresAt.After(clock.Now()))
	})

	t.Run("generates new config when cache is empty", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		s := newWindowsService(clock, client)

		config, err := s.loadTLSConfigForLDAP()
		require.NoError(t, err)
		require.NotNil(t, config)

		require.Equal(t, config, s.ldapTLSConfig)
		require.Equal(t, clock.Now().Add(tlsConfigCacheTTL), s.ldapTLSConfigExpiresAt)
	})

	t.Run("handles concurrent requests", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		s := newWindowsService(clock, client)

		s.ldapTLSConfig = &tls.Config{MinVersion: tls.VersionTLS10}
		s.ldapTLSConfigExpiresAt = clock.Now().Add(-1 * time.Hour)

		var wg sync.WaitGroup
		configs := make([]*tls.Config, 5)
		for i := range 5 {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				cfg, err := s.loadTLSConfigForLDAP()
				require.NoError(t, err)
				configs[idx] = cfg
			}(i)
		}
		wg.Wait()

		for _, cfg := range configs {
			require.NotNil(t, cfg)
		}
	})
}

func TestCRLUpdateSchedule(t *testing.T) {
	t.Parallel()

	const clusterName = "zarq"
	clock := clockwork.NewFakeClock()
	testAuth, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		ClusterName: clusterName,
		Clock:       clock,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, testAuth.Close()) })

	var runCRLLoopWG sync.WaitGroup
	// IMPORTANT! Must t.Cleanup before "cancel" (ie, cancel() needs to happen
	// first).
	t.Cleanup(func() {
		t.Log("Waiting for runCRLUpdateLoop() WaitGroup")
		runCRLLoopWG.Wait()
	})

	wsCtx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	caClient := newMockCertificateStoreClient(t)
	const publishInterval = 5 * time.Minute // Arbitrary. We use fake time.

	// Create a "fake" WindowsService instance. This only needs enough setup to do
	// runCRLUpdateLoop().
	winService := &WindowsService{
		cfg: WindowsServiceConfig{
			Logger:             logtest.NewLogger(),
			Clock:              clock,
			AccessPoint:        testAuth.AuthServer,
			PublishCRLInterval: publishInterval,
		},
		// Mock the actual CRL publishing.
		ca: caClient,
		// Short-circuit the "loadTLSConfigForLDAPlogic.
		ldapTLSConfig:          &tls.Config{},
		ldapTLSConfigExpiresAt: clock.Now().Add(1000000 * time.Hour), // Arbitrary. "Never" expires.
		// ctx for background methods.
		closeCtx: wsCtx,
		close:    cancel,
	}

	runCRLLoopWG.Go(func() {
		t.Log("Calling runCRLUpdateLoop()")
		winService.runCRLUpdateLoop()
	})

	var wantUpdates int
	waitForNextCRLUpdate := func(t *testing.T) {
		wantUpdates++
		caClient.WaitForUpdate(t, wantUpdates)
	}

	// First run of the loop invokes the update right away.
	waitForNextCRLUpdate(t)

	t.Run("update by elapsed time", func(t *testing.T) {
		// Don't t.Parallel().

		clock.Advance(publishInterval)
		waitForNextCRLUpdate(t)
	})

	t.Run("update by CA event", func(t *testing.T) {
		// Don't t.Parallel().

		ctx := t.Context()
		authServer := testAuth.AuthServer

		// Fetch current WindowsCA.
		id := types.CertAuthID{
			Type:       types.WindowsCA,
			DomainName: clusterName,
		}
		ca, err := authServer.GetCertAuthority(ctx, id, true /* loadKeys */)
		require.NoError(t, err)

		// Simulate a rotation by addding an entry to AdditionalTrustedKeys.
		keyPEM, certPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{
			Organization: []string{clusterName},
			CommonName:   clusterName,
		}, nil /* dnsNames */, 1*time.Hour /* ttl */)
		require.NoError(t, err)
		atk := ca.GetAdditionalTrustedKeys()
		atk.TLS = append(atk.TLS, &types.TLSKeyPair{
			Cert: certPEM,
			Key:  keyPEM,
			CRL:  []byte("fake CRL"),
		})
		require.NoError(t, ca.SetAdditionalTrustedKeys(atk))

		// Update. This generates a CA event.
		t.Log("Calling UpdateCertAuthority")
		_, err = authServer.UpdateCertAuthority(ctx, ca)
		require.NoError(t, err)

		waitForNextCRLUpdate(t)
	})
}

type mockCertificateStoreClient struct {
	logf func(string, ...any)

	mu       sync.Mutex
	wait     chan struct{} // waits on the next numCalls update
	numCalls int
}

func newMockCertificateStoreClient(t *testing.T) *mockCertificateStoreClient {
	c := &mockCertificateStoreClient{
		logf: t.Logf,
		wait: make(chan struct{}),
	}
	return c
}

func (c *mockCertificateStoreClient) Update(ctx context.Context, tc *tls.Config) error {
	c.mu.Lock()
	c.numCalls++
	close(c.wait)
	c.wait = make(chan struct{})
	c.mu.Unlock()
	return nil
}

func (c *mockCertificateStoreClient) WaitForUpdate(t *testing.T, wantCalls int) {
	// Arbitrary. 1s should be plenty of time for a mocked update.
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	for {
		c.mu.Lock()
		if c.numCalls == wantCalls {
			c.mu.Unlock()
			return
		}
		ch := c.wait
		c.mu.Unlock()

		select {
		case <-ctx.Done():
			t.Fatal("Timed out before update")
		case <-ch:
			continue
		}
	}
}
