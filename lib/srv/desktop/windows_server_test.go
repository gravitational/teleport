// Copyright 2021 Gravitational, Inc
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

package desktop

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"io"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/windows"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

func TestConfigWildcardBaseDN(t *testing.T) {
	cfg := &WindowsServiceConfig{
		DiscoveryBaseDN: "*",
		LDAPConfig: windows.LDAPConfig{
			Domain: "test.goteleport.com",
		},
	}
	require.NoError(t, cfg.checkAndSetDiscoveryDefaults())
	require.Equal(t, "DC=test,DC=goteleport,DC=com", cfg.DiscoveryBaseDN)
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
				DiscoveryBaseDN:      test.baseDN,
				DiscoveryLDAPFilters: test.filters,
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

	testSid := "S-1-5-21-1329593140-2634913955-1900852804-500"

	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
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

	client, err := tlsServer.NewClient(auth.TestServerID(types.RoleWindowsDesktop, "test-host-id"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	w := &WindowsService{
		clusterName: clusterName,
		cfg: WindowsServiceConfig{
			LDAPConfig: windows.LDAPConfig{
				Domain: domain,
			},
			AuthClient: client,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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
			activeDirectorySID: testSid,
		},
	} {
		certb, keyb, err := w.generateCredentials(ctx, generateCredentialsRequest{
			username:           user,
			domain:             domain,
			ttl:                windows.CertTTL,
			activeDirectorySID: test.activeDirectorySID,
		})
		require.NoError(t, err)
		require.NotNil(t, certb)
		require.NotNil(t, keyb)

		cert, err := x509.ParseCertificate(certb)
		require.NoError(t, err)
		require.NotNil(t, cert)

		require.Equal(t, user, cert.Subject.CommonName)
		require.Contains(t, cert.CRLDistributionPoints,
			`ldap:///CN=test,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=example,DC=com?certificateRevocationList?base?objectClass=cRLDistributionPoint`)

		foundKeyUsage := false
		foundAltName := false
		foundAdUserMapping := false
		for _, extension := range cert.Extensions {
			switch {
			case extension.Id.Equal(windows.EnhancedKeyUsageExtensionOID):
				foundKeyUsage = true
			case extension.Id.Equal(windows.SubjectAltNameExtensionOID):
				foundAltName = true
			case extension.Id.Equal(windows.ADUserMappingExtensionOID):
				foundAdUserMapping = true
			}
		}
		require.True(t, foundKeyUsage)
		require.True(t, foundAltName)
		require.Equal(t, test.activeDirectorySID != "", foundAdUserMapping)
	}
}

func TestEmitsRecordingEventsOnSend(t *testing.T) {
	clock := clockwork.NewFakeClock()
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Clock: clock,
		},
	}
	emitter := &eventstest.MockEmitter{}

	// a fake PNG Frame message
	encoded := []byte{byte(tdp.TypePNGFrame), 0x01, 0x02}

	delay := func() int64 { return 0 }
	handler := s.makeTDPSendHandler(context.Background(), emitter, delay,
		nil, "session-1", "windows.example.com", &tdp.Conn{})

	// the handler accepts both the message structure and its encoded form,
	// but our logic only depends on the encoded form, so pass a nil message
	var msg tdp.Message
	handler(msg, encoded)

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
			Clock: clock,
			Log:   &logrus.Logger{Out: io.Discard},
		},
	}
	emitter := &eventstest.MockEmitter{}

	// a fake PNG Frame message, which is way too big to be legitimate
	maliciousPNG := make([]byte, libevents.MaxProtoMessageSizeBytes+1)
	rand.Read(maliciousPNG)
	maliciousPNG[0] = byte(tdp.TypePNGFrame)

	delay := func() int64 { return 0 }
	handler := s.makeTDPSendHandler(context.Background(), emitter, delay,
		nil, "session-1", "windows.example.com", &tdp.Conn{})

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
	emitter := &eventstest.MockEmitter{}

	delay := func() int64 { return 0 }
	handler := s.makeTDPReceiveHandler(context.Background(), emitter, delay,
		nil, "session-1", "windows.example.com", &tdp.Conn{})

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
	s, id, emitter := setup()
	handler := s.makeTDPReceiveHandler(context.Background(),
		emitter, func() int64 { return 0 },
		id, "session-0", "windows.example.com", &tdp.Conn{})

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
	require.Equal(t, "session-0", cs.SessionID)
	require.Equal(t, "windows.example.com", cs.DesktopAddr)
	require.Equal(t, s.clusterName, cs.ClusterName)
	require.Equal(t, start, cs.Time)
}

func TestEmitsClipboardReceiveEvents(t *testing.T) {
	s, id, emitter := setup()
	handler := s.makeTDPSendHandler(context.Background(),
		emitter, func() int64 { return 0 },
		id, "session-0", "windows.example.com", &tdp.Conn{})

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
	require.True(t, ok)
	require.Equal(t, int32(len(fakeClipboardData)), cs.Length)
	require.Equal(t, "session-0", cs.SessionID)
	require.Equal(t, "windows.example.com", cs.DesktopAddr)
	require.Equal(t, s.clusterName, cs.ClusterName)
	require.Equal(t, start, cs.Time)
}

// TestAuditCacheLifecycle confirms that the audit cache is properly
// initialized upon receipt of a tdp.SharedDirectoryAnnounce message,
// and properly cleaned up upon session end.
func TestAuditCacheLifecycle(t *testing.T) {
	s, id, emitter := setup()
	sid := "session-0"
	desktopAddr := "windows.example.com"
	testDirName := "test-dir"
	path := "test/path/test-file.txt"
	var did uint32 = 2
	var cid uint32 = 999
	var offset uint64 = 500
	var length uint32 = 1000
	recvHandler := s.makeTDPReceiveHandler(context.Background(),
		emitter, func() int64 { return 0 },
		id, sid, desktopAddr, &tdp.Conn{})
	sendHandler := s.makeTDPSendHandler(context.Background(),
		emitter, func() int64 { return 0 },
		id, sid, desktopAddr, &tdp.Conn{})

	// SharedDirectoryAnnounce initializes the nameCache.
	msg := tdp.SharedDirectoryAnnounce{
		DirectoryID: 2,
		Name:        testDirName,
	}
	recvHandler(msg)

	// Check than an initialized audit cache entry is created
	// for sessionID upon receipt of a tdp.SharedDirectoryAnnounce.
	entry, ok := s.auditCache.m[sessionID(sid)]
	require.True(t, ok)
	require.NotNil(t, entry.nameCache)
	require.NotNil(t, entry.readRequestCache)
	require.NotNil(t, entry.writeRequestCache)

	// Confirm that audit cache entry for sid
	// is in the expected state.
	require.Equal(t, 1, entry.totalItems())
	name, ok := s.auditCache.GetName(sessionID(sid), directoryID(did))
	require.True(t, ok)
	require.Equal(t, directoryName(testDirName), name)
	_, ok = s.auditCache.TakeReadRequestInfo(sessionID(sid), completionID(cid))
	require.False(t, ok)
	_, ok = s.auditCache.TakeWriteRequestInfo(sessionID(sid), completionID(cid))
	require.False(t, ok)

	// A SharedDirectoryReadRequest should add a corresponding entry in the readRequestCache.
	readReq := tdp.SharedDirectoryReadRequest{
		CompletionID: cid,
		DirectoryID:  did,
		Path:         path,
		Offset:       offset,
		Length:       length,
	}
	encoded, err := readReq.Encode()
	require.NoError(t, err)
	sendHandler(readReq, encoded)
	require.Equal(t, 2, entry.totalItems())

	// A SharedDirectoryWriteRequest should add a corresponding entry in the writeRequestCache.
	writeReq := tdp.SharedDirectoryWriteRequest{
		CompletionID:    cid,
		DirectoryID:     did,
		Path:            path,
		Offset:          offset,
		WriteDataLength: length,
	}
	encoded, err = writeReq.Encode()
	require.NoError(t, err)
	sendHandler(writeReq, encoded)
	require.Equal(t, 3, entry.totalItems())

	// Check that the readRequestCache was properly filled out.
	require.Contains(t, entry.readRequestCache, completionID(cid))

	// Check that the writeRequestCache was properly filled out.
	require.Contains(t, entry.writeRequestCache, completionID(cid))

	// SharedDirectoryReadResponse should cause the entry in the readRequestCache to be cleaned up.
	readRes := tdp.SharedDirectoryReadResponse{
		CompletionID:   cid,
		ErrCode:        tdp.ErrCodeNil,
		ReadDataLength: length,
		ReadData:       []byte{}, // irrelevant in this context
	}
	recvHandler(readRes)
	require.Equal(t, 2, entry.totalItems())

	// SharedDirectoryWriteResponse should cause the entry in the writeRequestCache to be cleaned up.
	writeRes := tdp.SharedDirectoryWriteResponse{
		CompletionID: cid,
		ErrCode:      tdp.ErrCodeNil,
		BytesWritten: length,
	}
	recvHandler(writeRes)
	require.Equal(t, 1, entry.totalItems())

	// Check that the readRequestCache was properly cleaned up.
	require.NotContains(t, entry.readRequestCache, completionID(cid))

	// Check that the writeRequestCache was properly cleaned up.
	require.NotContains(t, entry.writeRequestCache, completionID(cid))

	// Simulate a session end event, which should clean up the cache for sessionID(sid) entirely.
	s.onSessionEnd(
		context.Background(),
		s.cfg.Emitter,
		id,
		s.cfg.Clock.Now().UTC().Round(time.Millisecond),
		true,
		"Administrator",
		sid,
		&types.WindowsDesktopV3{},
	)

	// Confirm that the audit cache at sessionID(sid) was cleaned up.
	_, ok = s.auditCache.GetName(sessionID(sid), directoryID(did))
	require.False(t, ok)
	_, ok = s.auditCache.TakeReadRequestInfo(sessionID(sid), completionID(cid))
	require.False(t, ok)
	_, ok = s.auditCache.TakeWriteRequestInfo(sessionID(sid), completionID(cid))
	require.False(t, ok)
	require.NotContains(t, s.auditCache.m, sessionID(sid))
}
