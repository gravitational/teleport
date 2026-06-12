/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package git

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/fixtures"
)

type fakeAuthPreferenceGetter struct {
}

func (f fakeAuthPreferenceGetter) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}

type fakeGitHubUserCertGenerator struct {
	clock    clockwork.Clock
	checkTTL time.Duration
}

func (f fakeGitHubUserCertGenerator) GenerateGitHubUserCert(_ context.Context, input *integrationv1.GenerateGitHubUserCertRequest, _ ...grpc.CallOption) (*integrationv1.GenerateGitHubUserCertResponse, error) {
	if f.checkTTL != 0 && f.checkTTL != input.Ttl.AsDuration() {
		return nil, trace.CompareFailed("expect ttl %v but got %v", f.checkTTL, input.Ttl.AsDuration())
	}

	caSigner, err := apisshutils.MakeTestSSHCA()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(input.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert := &ssh.Certificate{
		// we have to use key id to identify teleport user
		KeyId:       input.KeyId,
		Key:         pubKey,
		ValidAfter:  uint64(f.clock.Now().Add(-time.Minute).Unix()),
		ValidBefore: uint64(f.clock.Now().Add(input.Ttl.AsDuration()).Unix()),
		CertType:    ssh.UserCert,
	}
	if err := cert.SignCert(rand.Reader, caSigner); err != nil {
		return nil, trace.Wrap(err)
	}
	return &integrationv1.GenerateGitHubUserCertResponse{
		AuthorizedKey: ssh.MarshalAuthorizedKey(cert),
	}, nil
}

func TestMakeGitHubSigner(t *testing.T) {
	clock := clockwork.NewFakeClock()
	server := makeGitServer(t, "org")

	tests := []struct {
		name       string
		config     GitHubSignerConfig
		checkError require.ErrorAssertionFunc
	}{
		{
			name: "success",
			config: GitHubSignerConfig{
				Server:               server,
				GitHubUserID:         "1234567",
				TeleportUser:         "alice",
				AuthPreferenceGetter: fakeAuthPreferenceGetter{},
				GitHubUserCertGenerator: fakeGitHubUserCertGenerator{
					clock:    clock,
					checkTTL: defaultGitHubUserCertTTL,
				},
				IdentityExpires: clock.Now().Add(time.Hour),
				Clock:           clock,
			},
			checkError: require.NoError,
		},
		{
			name: "success short ttl",
			config: GitHubSignerConfig{
				Server:               server,
				GitHubUserID:         "1234567",
				TeleportUser:         "alice",
				AuthPreferenceGetter: fakeAuthPreferenceGetter{},
				GitHubUserCertGenerator: fakeGitHubUserCertGenerator{
					clock:    clock,
					checkTTL: time.Minute,
				},
				IdentityExpires: clock.Now().Add(time.Minute),
				Clock:           clock,
			},
			checkError: require.NoError,
		},
		{
			name: "no GitHubUserID",
			config: GitHubSignerConfig{
				Server:               server,
				TeleportUser:         "alice",
				AuthPreferenceGetter: fakeAuthPreferenceGetter{},
				GitHubUserCertGenerator: fakeGitHubUserCertGenerator{
					clock:    clock,
					checkTTL: time.Minute,
				},
				IdentityExpires: clock.Now().Add(time.Minute),
				Clock:           clock,
			},
			checkError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), i...)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := MakeGitHubSigner(context.Background(), test.config)
			test.checkError(t, err)
		})
	}
}

type mockGitHubMetaAPIServer struct {
	*httptest.Server

	metaResponse []byte
}

func newMockGitHubMetaAPIServer(t *testing.T, keys ...ssh.PublicKey) *mockGitHubMetaAPIServer {
	t.Helper()

	marshaledKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		marshaledKeys = append(marshaledKeys, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))))
	}
	metaResponse, err := json.Marshal(map[string][]string{
		"ssh_keys": marshaledKeys,
	})
	require.NoError(t, err)

	m := &mockGitHubMetaAPIServer{
		metaResponse: metaResponse,
	}
	m.Server = httptest.NewServer(m)
	t.Cleanup(m.Server.Close)
	return m
}

func (m *mockGitHubMetaAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write(m.metaResponse)
}

func Test_githubKeyDownloader(t *testing.T) {
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(fixtures.SSHCAPublicKey))
	require.NoError(t, err)

	mockSuccessServer := newMockGitHubMetaAPIServer(t, publicKey)
	mockFailureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(mockFailureServer.Close)

	tests := []struct {
		name              string
		setup             func(d *githubKeyDownloader)
		checkRefreshError require.ErrorAssertionFunc
		expectGetCount    int
	}{
		{
			name: "success first fetch",
			setup: func(d *githubKeyDownloader) {
				d.apiEndpoint = mockSuccessServer.URL
			},
			checkRefreshError: require.NoError,
			expectGetCount:    1,
		},
		{
			name: "success update",
			setup: func(d *githubKeyDownloader) {
				d.apiEndpoint = mockSuccessServer.URL
				d.keys.Store(&[]ssh.PublicKey{publicKey, publicKey})
			},
			checkRefreshError: require.NoError,
			expectGetCount:    1,
		},
		{
			name: "failure should not override existing keys",
			setup: func(d *githubKeyDownloader) {
				d.apiEndpoint = mockFailureServer.URL
				d.keys.Store(&[]ssh.PublicKey{publicKey, publicKey})
			},
			checkRefreshError: require.Error,
			expectGetCount:    2,
		},
	}

	for _, test := range tests {
		d := newGitHubKeyDownloader()
		if test.setup != nil {
			test.setup(d)
		}

		test.checkRefreshError(t, d.refresh(context.Background()))

		keys, err := d.GetKnownKeys()
		require.NoError(t, err)
		require.Len(t, keys, test.expectGetCount)
	}
}
