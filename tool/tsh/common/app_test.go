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

package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/client"
	defaults2 "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func startDummyHTTPServer(t *testing.T, name string) string {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", name)
		_, _ = w.Write([]byte("hello"))
	}))

	srv.Start()

	t.Cleanup(func() {
		srv.Close()
	})

	return srv.URL
}

func TestAppLoginLeaf(t *testing.T) {
	// TODO(tener): changing ResyncInterval defaults speeds up the tests considerably.
	// 	It may be worth making the change global either for tests or production.
	// 	See also SetTestTimeouts() in integration/helpers/timeouts.go
	oldResyncInterval := defaults2.ResyncInterval
	defaults2.ResyncInterval = time.Millisecond * 100
	t.Cleanup(func() {
		defaults2.ResyncInterval = oldResyncInterval
	})

	isInsecure := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() {
		lib.SetInsecureDevMode(isInsecure)
	})

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	// TODO(tener): consider making this default for tests.
	configStorage := func(cfg *servicecfg.Config) {
		cfg.Auth.SessionRecordingConfig.SetMode(types.RecordOff)
		cfg.Auth.StorageConfig.Params["poll_stream_period"] = 50 * time.Millisecond
	}

	rootAuth, rootProxy := makeTestServers(t,
		withClusterName(t, "root"),
		withBootstrap(connector, alice),
		withConfig(configStorage),
	)
	event, err := rootAuth.WaitForEventTimeout(time.Second, service.ProxyReverseTunnelReady)
	require.NoError(t, err)
	tunnel, ok := event.Payload.(reversetunnelclient.Server)
	require.True(t, ok)

	rootAppURL := startDummyHTTPServer(t, "rootapp")
	rootAppServer := makeTestApplicationServer(t, rootProxy, servicecfg.App{Name: "rootapp", URI: rootAppURL})
	_, err = rootAppServer.WaitForEventTimeout(time.Second*10, service.TeleportReadyEvent)
	require.NoError(t, err)

	rootProxyAddr, err := rootProxy.ProxyWebAddr()
	require.NoError(t, err)
	rootTunnelAddr, err := rootProxy.ProxyTunnelAddr()
	require.NoError(t, err)

	trustedCluster, err := types.NewTrustedCluster("localhost", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{},
		Token:                staticToken,
		ProxyAddress:         rootProxyAddr.String(),
		ReverseTunnelAddress: rootTunnelAddr.String(),
		RoleMap: []types.RoleMapping{
			{
				Remote: "access",
				Local:  []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	leafAuth, leafProxy := makeTestServers(t, withClusterName(t, "leaf"), withConfig(configStorage))

	leafAppURL := startDummyHTTPServer(t, "leafapp")
	leafAppServer := makeTestApplicationServer(t, leafProxy, servicecfg.App{Name: "leafapp", URI: leafAppURL})
	_, err = leafAppServer.WaitForEventTimeout(time.Second*10, service.TeleportReadyEvent)
	require.NoError(t, err)

	tryCreateTrustedCluster(t, leafAuth.GetAuthServer(), trustedCluster)

	// wait for the connection to come online and the app server information propagate.
	require.Eventually(t, func() bool {
		conns, err := rootAuth.GetAuthServer().GetTunnelConnections("leaf")
		return err == nil && len(conns) == 1
	}, 10*time.Second, 100*time.Millisecond, "leaf cluster did not come online")

	require.Eventually(t, func() bool {
		leafSite, err := tunnel.GetSite("leaf")
		require.NoError(t, err)
		ap, err := leafSite.CachingAccessPoint()
		require.NoError(t, err)

		servers, err := ap.GetApplicationServers(context.Background(), defaults.Namespace)
		if err != nil {
			return false
		}
		return len(servers) == 1 && servers[0].GetName() == "leafapp"
	}, 10*time.Second, 100*time.Millisecond, "leaf cluster did not come online")

	// helpers
	getHelpers := func(t *testing.T) (func(cluster string) string, func(args ...string) string) {
		tmpHomePath := t.TempDir()

		run := func(args []string, opts ...CliOption) string {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			captureStdout := new(bytes.Buffer)
			opts = append(opts, setHomePath(tmpHomePath))
			opts = append(opts, setCopyStdout(captureStdout))
			err := Run(ctx, args, opts...)
			require.NoError(t, err)
			return captureStdout.String()
		}

		login := func(cluster string) string {
			args := []string{
				"login",
				"--insecure",
				"--debug",
				"--proxy", rootProxyAddr.String(),
				cluster,
			}

			opt := setMockSSOLogin(rootAuth.GetAuthServer(), alice, connector.GetName())

			return run(args, opt)
		}
		tsh := func(args ...string) string { return run(args) }

		return login, tsh
	}

	verifyAppIsAvailable := func(t *testing.T, conf string, appName string) {
		var info appConfigInfo
		require.NoError(t, json.Unmarshal([]byte(conf), &info))

		clientCert, err := tls.LoadX509KeyPair(info.Cert, info.Key)
		require.NoError(t, err)

		clt := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					Certificates:       []tls.Certificate{clientCert},
				},
			},
		}

		resp, err := clt.Get(fmt.Sprintf("https://%v", rootProxyAddr.Addr))
		require.NoError(t, err)

		respData, _ := httputil.DumpResponse(resp, true)

		t.Log(string(respData))

		require.Equal(t, 200, resp.StatusCode)
		require.Equal(t, appName, resp.Header.Get("Server"))
		_ = resp.Body.Close()
	}

	tests := []struct{ name, loginCluster, appCluster, appName string }{
		{"root login cluster, root app cluster", "root", "root", "rootapp"},
		{"root login cluster, leaf app cluster", "root", "leaf", "leafapp"},
		{"leaf login cluster, root app cluster", "leaf", "root", "rootapp"},
		{"leaf login cluster, leaf app cluster", "leaf", "leaf", "leafapp"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			login, tsh := getHelpers(t)

			login(tt.loginCluster)
			tsh("app", "ls", "--verbose", "--format=json", "--cluster", tt.appCluster)
			tsh("app", "login", tt.appName, "--cluster", tt.appCluster)
			conf := tsh("app", "config", "--format=json")
			verifyAppIsAvailable(t, conf, tt.appName)
			tsh("logout")
		})
	}
}

func TestFormatAppConfig(t *testing.T) {
	t.Parallel()

	defaultTc := &client.TeleportClient{
		Config: client.Config{
			WebProxyAddr: "test-tp.teleport:8443",
		},
	}
	testProfile := &client.ProfileStatus{
		Username: "test-user",
		Dir:      "/test/dir",
	}
	testAppName := "test-tp"
	testAppPublicAddr := "test-tp.teleport"
	testCluster := "test-tp"

	tests := []struct {
		name              string
		tc                *client.TeleportClient
		format            string
		awsArn            string
		azureIdentity     string
		gcpServiceAccount string
		insecure          bool
		expected          string
		wantErr           bool
	}{
		{
			name: "format URI standard HTTPS port",
			tc: &client.TeleportClient{
				Config: client.Config{
					WebProxyAddr: "test-tp.teleport:443",
				},
			},
			format:   appFormatURI,
			expected: "https://test-tp.teleport",
		},
		{
			name:     "format URI standard non-standard HTTPS port",
			tc:       defaultTc,
			format:   appFormatURI,
			expected: "https://test-tp.teleport:8443",
		},
		{
			name:     "format CA",
			tc:       defaultTc,
			format:   appFormatCA,
			expected: "/test/dir/keys/cas/test-tp.pem",
		},
		{
			name:     "format cert",
			tc:       defaultTc,
			format:   appFormatCert,
			expected: "/test/dir/keys/test-user-app/test-tp-x509.pem",
		},
		{
			name:     "format key",
			tc:       defaultTc,
			format:   appFormatKey,
			expected: "/test/dir/keys/test-user",
		},
		{
			name:   "format curl standard non-standard HTTPS port",
			tc:     defaultTc,
			format: appFormatCURL,
			expected: `curl \
  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
  --key /test/dir/keys/test-user \
  https://test-tp.teleport:8443`,
		},
		{
			name:     "format insecure curl standard non-standard HTTPS port",
			tc:       defaultTc,
			format:   appFormatCURL,
			insecure: true,
			expected: `curl --insecure \
  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
  --key /test/dir/keys/test-user \
  https://test-tp.teleport:8443`,
		},
		{
			name:   "format JSON",
			tc:     defaultTc,
			format: appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443"
}
`,
		},
		{
			name:   "format YAML",
			tc:     defaultTc,
			format: appFormatYAML,
			expected: `ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
		{
			name:   "format default",
			tc:     defaultTc,
			format: "default",
			expected: `Name:      test-tp                                       
URI:       https://test-tp.teleport:8443                 
CA:        /test/dir/keys/cas/test-tp.pem                
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:       /test/dir/keys/test-user                      
`,
		},
		{
			name:   "empty format means default",
			tc:     defaultTc,
			format: "",
			expected: `Name:      test-tp                                       
URI:       https://test-tp.teleport:8443                 
CA:        /test/dir/keys/cas/test-tp.pem                
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:       /test/dir/keys/test-user                      
`,
		},
		{
			name:    "reject invalid format",
			tc:      defaultTc,
			format:  "invalid",
			wantErr: true,
		},
		// Azure
		{
			name:          "azure default format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        "default",
			expected: `Name:      test-tp                                                                                                                                                        
URI:       https://test-tp.teleport:8443                                                                                                                                  
CA:        /test/dir/keys/cas/test-tp.pem                                                                                                                                 
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem                                                                                                                  
Key:       /test/dir/keys/test-user                                                                                                                                       
Azure Id:  /subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure 
`,
		},
		{
			name:          "azure JSON format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443",
  "azure_identity": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"
}
`,
		},
		{
			name:          "azure YAML format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        appFormatYAML,
			expected: `azure_identity: /subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure
ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
		// GCP
		{
			name:              "gcp default format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            "default",
			expected: `Name:                test-tp                                       
URI:                 https://test-tp.teleport:8443                 
CA:                  /test/dir/keys/cas/test-tp.pem                
Cert:                /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:                 /test/dir/keys/test-user                      
GCP Service Account: dev@example-123456.iam.gserviceaccount.com    
`,
		},
		{
			name:              "gcp JSON format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443",
  "gcp_service_account": "dev@example-123456.iam.gserviceaccount.com"
}
`,
		},
		{
			name:              "gcp YAML format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            appFormatYAML,
			expected: `ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
gcp_service_account: dev@example-123456.iam.gserviceaccount.com
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.tc.InsecureSkipVerify = test.insecure
			result, err := formatAppConfig(test.tc, testProfile, testAppName, testAppPublicAddr, test.format, testCluster, test.awsArn, test.azureIdentity, test.gcpServiceAccount)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}
