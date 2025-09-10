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
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base32"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authtest"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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

	ca, err := authServer.AuthServer.GetCertAuthorities(t.Context(), types.UserCA, false)
	require.NoError(t, err)
	require.Len(t, ca, 1)

	keys := ca[0].GetActiveKeys()
	require.Len(t, keys.TLS, 1)

	cert, err := tlsca.ParseCertificatePEM(keys.TLS[0].Cert)
	require.NoError(t, err)
	commonName := base32.HexEncoding.EncodeToString(cert.SubjectKeyId) + "_" + clusterName

	client, err := tlsServer.NewClient(authtest.TestServerID(types.RoleWindowsDesktop, "test-host-id"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testSID := "S-1-5-21-1329593140-2634913955-1900852804-500"

	for _, test := range []struct {
		name               string
		activeDirectorySID string
	}{
		{
			name:               "no ad sid",
			activeDirectorySID: "",
		},
		{
			name:               "with ad sid",
			activeDirectorySID: testSID,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			certb, keyb, err := winpki.GenerateWindowsDesktopCredentials(ctx, client, &winpki.GenerateCredentialsRequest{
				Username:           user,
				Domain:             domain,
				TTL:                5 * time.Minute,
				ClusterName:        clusterName,
				ActiveDirectorySID: test.activeDirectorySID,
			})
			require.NoError(t, err)
			require.NotNil(t, certb)
			require.NotNil(t, keyb)

			cert, err := x509.ParseCertificate(certb)
			require.NoError(t, err)
			require.NotNil(t, cert)

			require.Equal(t, user, cert.Subject.CommonName)
			require.Contains(t, cert.CRLDistributionPoints,
				`ldap:///CN=`+commonName+`,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=example,DC=com?certificateRevocationList?base?objectClass=cRLDistributionPoint`)

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

func TestEmitsRecordingEventsOnSend(t *testing.T) {
	clock := clockwork.NewFakeClock()
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Clock: clock,
		},
	}
	emitter := &eventstest.MockRecorderEmitter{}
	emitterPreparer := libevents.WithNoOpPreparer(emitter)

	// a fake PNG Frame message
	encoded := []byte{byte(tdp.TypePNGFrame), 0x01, 0x02}

	delay := func() int64 { return 0 }
	handler := s.makeTDPSendHandler(context.Background(), emitterPreparer, delay, nil /* conn */, nil /* auditor */)

	// the handler accepts both the message structure and its encoded form,
	// but our logic only depends on the encoded form, so pass a nil message
	handler(nil /* message */, encoded)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	dr, ok := e.(*events.DesktopRecording)
	require.True(t, ok)
	require.Equal(t, encoded, dr.Message)
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
	maliciousPNG[0] = byte(tdp.TypePNGFrame)

	delay := func() int64 { return 0 }
	handler := s.makeTDPSendHandler(context.Background(), emitterPreparer, delay, nil /* conn */, nil /* auditor */)

	// the handler accepts both the message structure and its encoded form,
	// but our logic only depends on the encoded form, so pass a nil message
	var msg tdp.Message
	handler(msg, maliciousPNG)

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

	msg := tdp.MouseButton{
		Button: tdp.LeftMouseButton,
		State:  tdp.ButtonPressed,
	}
	handler(msg)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	dr, ok := e.(*events.DesktopRecording)
	require.True(t, ok)
	decoded, err := tdp.Decode(dr.Message)
	require.NoError(t, err)
	require.Equal(t, msg, decoded)
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
	msg := tdp.ClipboardData(fakeClipboardData)
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
	msg := tdp.ClipboardData(fakeClipboardData)
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
