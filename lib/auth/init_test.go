/*
Copyright 2015-2021 Gravitational, Inc.

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

package auth

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

// TestReadIdentity makes parses identity from private key and certificate
// and checks that all parameters are valid
func TestReadIdentity(t *testing.T) {
	clock := clockwork.NewFakeClock()
	a := testauthority.NewWithClock(clock)
	priv, pub, err := a.GenerateKeyPair("")
	require.NoError(t, err)

	cert, err := a.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		CASigningAlg:        defaults.CASignatureAlgorithm,
		PublicHostKey:       pub,
		HostID:              "id1",
		NodeName:            "node-name",
		ClusterName:         "example.com",
		Roles:               teleport.Roles{teleport.RoleNode},
		TTL:                 0,
	})
	require.NoError(t, err)

	id, err := ReadSSHIdentityFromKeyPair(priv, cert)
	require.NoError(t, err)
	require.Equal(t, id.ClusterName, "example.com")
	require.Equal(t, id.ID, IdentityID{HostUUID: "id1.example.com", Role: teleport.RoleNode})
	require.Equal(t, id.CertBytes, cert)
	require.Equal(t, id.KeyBytes, priv)

	// test TTL by converting the generated cert to text -> back and making sure ExpireAfter is valid
	ttl := 10 * time.Second
	expiryDate := clock.Now().Add(ttl)
	bytes, err := a.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		CASigningAlg:        defaults.CASignatureAlgorithm,
		PublicHostKey:       pub,
		HostID:              "id1",
		NodeName:            "node-name",
		ClusterName:         "example.com",
		Roles:               teleport.Roles{teleport.RoleNode},
		TTL:                 ttl,
	})
	require.NoError(t, err)
	pk, _, _, _, err := ssh.ParseAuthorizedKey(bytes)
	require.NoError(t, err)
	copy, ok := pk.(*ssh.Certificate)
	require.True(t, ok)
	require.Equal(t, uint64(expiryDate.Unix()), copy.ValidBefore)
}

func TestBadIdentity(t *testing.T) {
	a := testauthority.New()
	priv, pub, err := a.GenerateKeyPair("")
	require.NoError(t, err)

	// bad cert type
	_, err = ReadSSHIdentityFromKeyPair(priv, pub)
	require.IsType(t, trace.BadParameter(""), err)

	// missing authority domain
	cert, err := a.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		CASigningAlg:        defaults.CASignatureAlgorithm,
		PublicHostKey:       pub,
		HostID:              "id2",
		NodeName:            "",
		ClusterName:         "",
		Roles:               teleport.Roles{teleport.RoleNode},
		TTL:                 0,
	})
	require.NoError(t, err)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)

	// missing host uuid
	cert, err = a.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		CASigningAlg:        defaults.CASignatureAlgorithm,
		PublicHostKey:       pub,
		HostID:              "example.com",
		NodeName:            "",
		ClusterName:         "",
		Roles:               teleport.Roles{teleport.RoleNode},
		TTL:                 0,
	})
	require.NoError(t, err)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)

	// unrecognized role
	cert, err = a.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		CASigningAlg:        defaults.CASignatureAlgorithm,
		PublicHostKey:       pub,
		HostID:              "example.com",
		NodeName:            "",
		ClusterName:         "id1",
		Roles:               teleport.Roles{teleport.Role("bad role")},
		TTL:                 0,
	})
	require.NoError(t, err)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	require.IsType(t, trace.BadParameter(""), err)
}

// TestAuthPreference ensures that the act of creating an AuthServer sets
// the AuthPreference (type and second factor) on the backend.
func TestAuthPreference(t *testing.T) {
	tempDir := t.TempDir()

	bk, err := lite.New(context.TODO(), backend.Params{"path": tempDir})
	require.NoError(t, err)

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "u2f",
		U2F: &services.U2F{
			AppID:  "foo",
			Facets: []string{"bar", "baz"},
		},
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{},
	})
	require.NoError(t, err)

	ac := InitConfig{
		DataDir:        tempDir,
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		ClusterConfig:  services.DefaultClusterConfig(),
		ClusterName:    clusterName,
		StaticTokens:   staticTokens,
		AuthPreference: ap,
	}
	as, err := Init(ac)
	require.NoError(t, err)
	defer as.Close()

	cap, err := as.GetAuthPreference()
	require.NoError(t, err)
	require.Equal(t, cap.GetType(), "local")
	require.Equal(t, cap.GetSecondFactor(), constants.SecondFactorU2F)

	u, err := cap.GetU2F()
	require.NoError(t, err)
	require.Equal(t, u.AppID, "foo")
	require.Equal(t, u.Facets, []string{"bar", "baz"})
}

func TestClusterID(t *testing.T) {
	tempDir := t.TempDir()

	bk, err := lite.New(context.TODO(), backend.Params{"path": tempDir})
	require.NoError(t, err)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type: "local",
	})
	require.NoError(t, err)

	authServer, err := Init(InitConfig{
		DataDir:        t.TempDir(),
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		ClusterConfig:  services.DefaultClusterConfig(),
		ClusterName:    clusterName,
		StaticTokens:   services.DefaultStaticTokens(),
		AuthPreference: authPreference,
	})
	require.NoError(t, err)
	defer authServer.Close()

	cc, err := authServer.GetClusterConfig()
	require.NoError(t, err)
	clusterID := cc.GetClusterID()
	require.NotEqual(t, clusterID, "")

	// do it again and make sure cluster ID hasn't changed
	authServer, err = Init(InitConfig{
		DataDir:        t.TempDir(),
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		ClusterConfig:  services.DefaultClusterConfig(),
		ClusterName:    clusterName,
		StaticTokens:   services.DefaultStaticTokens(),
		AuthPreference: authPreference,
	})
	require.NoError(t, err)
	defer authServer.Close()

	cc, err = authServer.GetClusterConfig()
	require.NoError(t, err)
	require.Equal(t, cc.GetClusterID(), clusterID)
}

// TestClusterName ensures that a cluster can not be renamed.
func TestClusterName(t *testing.T) {
	bk, err := lite.New(context.TODO(), backend.Params{"path": t.TempDir()})
	require.NoError(t, err)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type: "local",
	})
	require.NoError(t, err)

	authServer, err := Init(InitConfig{
		DataDir:        t.TempDir(),
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		ClusterConfig:  services.DefaultClusterConfig(),
		ClusterName:    clusterName,
		StaticTokens:   services.DefaultStaticTokens(),
		AuthPreference: authPreference,
	})
	require.NoError(t, err)
	defer authServer.Close()

	// Start the auth server with a different cluster name. The auth server
	// should start, but with the original name.
	clusterName, err = services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "dev.localhost",
	})
	require.NoError(t, err)

	authServer, err = Init(InitConfig{
		DataDir:        t.TempDir(),
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		ClusterConfig:  services.DefaultClusterConfig(),
		ClusterName:    clusterName,
		StaticTokens:   services.DefaultStaticTokens(),
		AuthPreference: authPreference,
	})
	require.NoError(t, err)
	defer authServer.Close()

	cn, err := authServer.GetClusterName()
	require.NoError(t, err)
	require.Equal(t, cn.GetClusterName(), "me.localhost")
}

func TestCASigningAlg(t *testing.T) {
	bk, err := lite.New(context.TODO(), backend.Params{"path": t.TempDir()})
	require.NoError(t, err)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type: "local",
	})
	require.NoError(t, err)

	conf := InitConfig{
		DataDir:        t.TempDir(),
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		ClusterConfig:  services.DefaultClusterConfig(),
		ClusterName:    clusterName,
		StaticTokens:   services.DefaultStaticTokens(),
		AuthPreference: authPreference,
	}

	verifyCAs := func(auth *Server, alg string) {
		hostCAs, err := auth.GetCertAuthorities(services.HostCA, false)
		require.NoError(t, err)
		for _, ca := range hostCAs {
			require.Equal(t, sshutils.GetSigningAlgName(ca), alg)
		}
		userCAs, err := auth.GetCertAuthorities(services.UserCA, false)
		require.NoError(t, err)
		for _, ca := range userCAs {
			require.Equal(t, sshutils.GetSigningAlgName(ca), alg)
		}
	}

	// Start a new server without specifying a signing alg.
	auth, err := Init(conf)
	require.NoError(t, err)
	verifyCAs(auth, ssh.SigAlgoRSASHA2512)

	require.NoError(t, auth.Close())

	// Reset the auth server state.
	conf.Backend, err = lite.New(context.TODO(), backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	conf.DataDir = t.TempDir()

	// Start a new server with non-default signing alg.
	signingAlg := ssh.SigAlgoRSA
	conf.CASigningAlg = &signingAlg
	auth, err = Init(conf)
	require.NoError(t, err)
	defer auth.Close()
	verifyCAs(auth, ssh.SigAlgoRSA)

	// Start again, using a different alg. This should not change the existing
	// CA.
	signingAlg = ssh.SigAlgoRSASHA2256
	auth, err = Init(conf)
	require.NoError(t, err)
	verifyCAs(auth, ssh.SigAlgoRSA)
}

// TestIdentityChecker verifies auth identity properly validates host
// certificates when connecting to an SSH server.
func TestIdentityChecker(t *testing.T) {
	bk, err := lite.New(context.TODO(), backend.Params{"path": t.TempDir()})
	require.NoError(t, err)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type: "local",
	})
	require.NoError(t, err)

	conf := InitConfig{
		DataDir:        t.TempDir(),
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		ClusterConfig:  services.DefaultClusterConfig(),
		ClusterName:    clusterName,
		StaticTokens:   services.DefaultStaticTokens(),
		AuthPreference: authPreference,
	}

	authServer, err := Init(conf)
	require.NoError(t, err)
	t.Cleanup(func() { authServer.Close() })

	ca, err := authServer.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: "me.localhost",
	}, true)
	require.NoError(t, err)

	signers, err := sshutils.GetSigners(ca)
	require.NoError(t, err)
	require.Len(t, signers, 1)

	realCert, err := sshutils.MakeRealHostCert(signers[0])
	require.NoError(t, err)

	spoofedCert, err := sshutils.MakeSpoofedHostCert(signers[0])
	require.NoError(t, err)

	tests := []struct {
		desc string
		cert ssh.Signer
		err  bool
	}{
		{
			desc: "should be able to connect with real cert",
			cert: realCert,
			err:  false,
		},
		{
			desc: "should not be able to connect with spoofed cert",
			cert: spoofedCert,
			err:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			handler := sshutils.NewChanHandlerFunc(func(_ context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
				ch, _, err := nch.Accept()
				require.NoError(t, err)
				require.NoError(t, ch.Close())
			})
			sshServer, err := sshutils.NewServer(
				"test",
				utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
				handler,
				[]ssh.Signer{test.cert},
				sshutils.AuthMethods{NoClient: true},
				sshutils.SetInsecureSkipHostValidation(),
			)
			require.NoError(t, err)
			t.Cleanup(func() { sshServer.Close() })
			require.NoError(t, sshServer.Start())

			identity, err := GenerateIdentity(authServer, IdentityID{
				Role:     teleport.RoleNode,
				HostUUID: uuid.New(),
				NodeName: "node-1",
			}, nil, nil)
			require.NoError(t, err)

			sshClientConfig, err := identity.SSHClientConfig(false)
			require.NoError(t, err)

			dialer := proxy.DialerFromEnvironment(sshServer.Addr())
			sconn, err := dialer.Dial("tcp", sshServer.Addr(), sshClientConfig)
			if test.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NoError(t, sconn.Close())
			}
		})
	}
}
