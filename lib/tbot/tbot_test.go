/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tbot

import (
	"context"
	"crypto/rsa"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/native"
	libconfig "github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	native.PrecomputeTestKeys(m)
	os.Exit(m.Run())
}

// TestBot is a one-shot run of the bot that communicates with a stood up
// in memory auth server.
//
// Assertions here should focus on the content of output credentials,
// rather than their exact formatting. Tests that check the exact formatting
// of rendered destinations should be kept closer to the implementation.
//
// This test suite should assume the auth server/proxy is well-behaved
func TestBot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewLoggerForTests()

	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	const appName = "foo"
	fc.Apps = libconfig.Apps{
		Service: libconfig.Service{
			EnabledFlag: "true",
		},
		Apps: []*libconfig.App{
			{
				Name:       appName,
				PublicAddr: "foo.example.com",
				URI:        "http://foo.example.com:1234",
			},
		},
	}
	fc.Databases = libconfig.Databases{
		Service: libconfig.Service{
			EnabledFlag: "true",
		},
		Databases: []*libconfig.Database{
			{
				Name:     "foo",
				Protocol: "mysql",
				URI:      "foo.example.com:1234",
			},
		},
	}

	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)

	// Wait for the app/db to become available. Sometimes this takes a bit
	// of time in CI.
	require.Eventually(t, func() bool {
		_, err := getApp(ctx, rootClient, appName)
		if err != nil {
			return false
		}
		_, err = getDatabase(ctx, rootClient, "foo")
		return err == nil
	}, 10*time.Second, 100*time.Millisecond)

	const roleName = "output-role"
	hostCertRule := types.NewRule("host_cert", []string{"create"})
	hostCertRule.Where = "is_subset(host_cert.principals, \"nodename.my.domain.com\")"
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},

			// Note: we don't actually need a role granting us database access to
			// request it. Actual access is validated via RBAC at connection time.
			// We do need an actual database and permission to list them, however.
			DatabaseLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			DatabaseNames: []string{"bar"},
			DatabaseUsers: []string{"baz"},
			Rules: []types.Rule{
				types.NewRule("db_server", []string{"read", "list"}),
				hostCertRule,
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, rootClient.UpsertRole(ctx, role))

	// Make and join a new bot instance.
	botParams := testhelpers.MakeBot(t, rootClient, "test", roleName)

	identityOutput := &config.IdentityOutput{
		Common: config.OutputCommon{
			Destination: config.WrapDestination(&config.DestinationMemory{}),
		},
	}
	appOutput := &config.ApplicationOutput{
		Common: config.OutputCommon{
			Destination: config.WrapDestination(&config.DestinationMemory{}),
		},
		AppName: appName,
	}
	dbOutput := &config.DatabaseOutput{
		Common: config.OutputCommon{
			Destination: config.WrapDestination(&config.DestinationMemory{}),
		},
		Service:  "foo",
		Database: "bar",
		Username: "baz",
	}
	sshHostOutput := &config.SSHHostOutput{
		Common: config.OutputCommon{
			Destination: config.WrapDestination(&config.DestinationMemory{}),
		},
		Principals: []string{"nodename.my.domain.com"},
	}
	botConfig := testhelpers.MakeMemoryBotConfig(
		t, fc, botParams, []config.Output{
			identityOutput,
			appOutput,
			dbOutput,
			sshHostOutput,
		},
	)
	b := New(botConfig, log)
	require.NoError(t, b.Run(ctx))

	t.Run("validate bot identity", func(t *testing.T) {
		// Some rough checks to ensure the bot identity used follows our
		// expected rules for bot identities.
		botIdent := b.ident()
		tlsIdent, err := tlsca.FromSubject(botIdent.X509Cert.Subject, botIdent.X509Cert.NotAfter)
		require.NoError(t, err)
		require.True(t, tlsIdent.Renewable)
		require.False(t, tlsIdent.DisallowReissue)
		require.Equal(t, uint64(1), tlsIdent.Generation)
		require.ElementsMatch(t, []string{botParams.RoleName}, tlsIdent.Groups)
	})

	t.Run("output: identity", func(t *testing.T) {
		route := tlsIdentFromDest(t, appOutput.GetDestination()).RouteToApp
		require.Equal(t, appName, route.Name)
		require.Equal(t, "foo.example.com", route.PublicAddr)
		require.NotEmpty(t, route.SessionID)
	})

	t.Run("output: application", func(t *testing.T) {
		route := tlsIdentFromDest(t, appOutput.GetDestination()).RouteToApp
		require.Equal(t, appName, route.Name)
		require.Equal(t, "foo.example.com", route.PublicAddr)
		require.NotEmpty(t, route.SessionID)
	})

	t.Run("output: database", func(t *testing.T) {
		route := tlsIdentFromDest(t, dbOutput.GetDestination()).RouteToDatabase
		require.Equal(t, "foo", route.ServiceName)
		require.Equal(t, "bar", route.Database)
		require.Equal(t, "baz", route.Username)
		require.Equal(t, "mysql", route.Protocol)
	})

	t.Run("output: ssh_host", func(t *testing.T) {
		dest := sshHostOutput.GetDestination()

		// Validate ssh_host
		keyBytes, err := dest.Read("ssh_host")
		require.NoError(t, err)
		privateKey, err := ssh.ParseRawPrivateKey(keyBytes)
		require.NoError(t, err)
		rsaPrivateKey := privateKey.(*rsa.PrivateKey)

		// Validate ssh_host-cert.pub
		certBytes, err := dest.Read("ssh_host-cert.pub")
		require.NoError(t, err)
		cert, err := sshutils.ParseCertificate(certBytes)
		require.NoError(t, err)
		certPublicKey := cert.Key.(ssh.CryptoPublicKey).CryptoPublicKey()
		require.True(
			t,
			rsaPrivateKey.PublicKey.Equal(certPublicKey),
			"the public key of the host cert should match the private key",
		)

		_, err = dest.Read("ssh_host-user-ca.pub")
		require.NoError(t, err)
	})
}

func tlsIdentFromDest(t *testing.T, dest bot.Destination) *tlsca.Identity {
	t.Helper()
	keyBytes, err := dest.Read(identity.PrivateKeyKey)
	require.NoError(t, err)
	certBytes, err := dest.Read(identity.TLSCertKey)
	require.NoError(t, err)
	hostCABytes, err := dest.Read(config.HostCAPath)
	require.NoError(t, err)
	ident := &identity.Identity{}
	err = identity.ReadTLSIdentityFromKeyPair(ident, keyBytes, certBytes, [][]byte{hostCABytes})
	require.NoError(t, err)

	tlsIdent, err := tlsca.FromSubject(
		ident.X509Cert.Subject, ident.X509Cert.NotAfter,
	)
	require.NoError(t, err)
	return tlsIdent
}
