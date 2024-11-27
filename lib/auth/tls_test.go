/*
Copyright 2017-2020 Gravitational, Inc.

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
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	eventtypes "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestRejectedClients(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_REJECT_OLD_CLIENTS", "yes")

	server, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         t.TempDir(),
		ClusterName: "cluster",
		Clock:       clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	user, _, err := CreateUserAndRole(server.AuthServer, "user", []string{"role"}, nil)
	require.NoError(t, err)

	tlsServer, err := server.NewTestTLSServer()
	require.NoError(t, err)
	defer tlsServer.Close()

	tlsConfig, err := tlsServer.ClientTLSConfig(TestUser(user.GetName()))
	require.NoError(t, err)

	clt, err := authclient.NewClient(client.Config{
		DialInBackground: true,
		Addrs:            []string{tlsServer.Addr().String()},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: breaker.NoopBreakerConfig(),
	})
	require.NoError(t, err)
	defer clt.Close()

	t.Run("reject old version", func(t *testing.T) {
		version := teleport.MinClientSemVersion
		version.Major--
		ctx := context.WithValue(context.Background(), metadata.DisableInterceptors{}, struct{}{})
		ctx = metadata.AddMetadataToContext(ctx, map[string]string{
			metadata.VersionKey: version.String(),
		})
		resp, err := clt.Ping(ctx)
		require.True(t, trace.IsConnectionProblem(err))
		require.Equal(t, proto.PingResponse{}, resp)
	})

	t.Run("allow valid versions", func(t *testing.T) {
		version := teleport.MinClientSemVersion
		version.Major--
		for i := 0; i < 5; i++ {
			version.Major++

			ctx := context.WithValue(context.Background(), metadata.DisableInterceptors{}, struct{}{})
			ctx = metadata.AddMetadataToContext(ctx, map[string]string{
				metadata.VersionKey: version.String(),
			})
			resp, err := clt.Ping(ctx)
			require.NoError(t, err)
			require.NotNil(t, resp)
		}
	})
}

// TestRemoteBuiltinRole tests remote builtin role
// that gets mapped to remote proxy readonly role
func TestRemoteBuiltinRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         t.TempDir(),
		ClusterName: "remote",
		Clock:       testSrv.AuthServer.TestAuthServerConfig.Clock,
	})
	require.NoError(t, err)

	certPool, err := testSrv.CertPool()
	require.NoError(t, err)

	// without trust, proxy server will get rejected
	// remote auth server will get rejected because it is not supported
	remoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleProxy), testSrv.Addr(), certPool)
	require.NoError(t, err)

	// certificate authority is not recognized, because
	// the trust has not been established yet
	_, err = remoteProxy.GetNodes(ctx, apidefaults.Namespace)
	require.True(t, trace.IsConnectionProblem(err))

	// after trust is established, things are good
	err = testSrv.AuthServer.Trust(ctx, remoteServer, nil)
	require.NoError(t, err)

	// re initialize client with trust established.
	remoteProxy, err = remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleProxy), testSrv.Addr(), certPool)
	require.NoError(t, err)

	_, err = remoteProxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// remote auth server will get rejected even with established trust
	remoteAuth, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleAuth), testSrv.Addr(), certPool)
	require.NoError(t, err)

	_, err = remoteAuth.GetDomainName(ctx)
	require.True(t, trace.IsAccessDenied(err))
}

// TestAcceptedUsage tests scenario when server is set up
// to accept certificates with certain usage metadata restrictions
// encoded
func TestAcceptedUsage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	server, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:           t.TempDir(),
		ClusterName:   "remote",
		AcceptedUsage: []string{"usage:k8s"},
		Clock:         clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	user, _, err := CreateUserAndRole(server.AuthServer, "user", []string{"role"}, nil)
	require.NoError(t, err)

	tlsServer, err := server.NewTestTLSServer()
	require.NoError(t, err)
	defer tlsServer.Close()

	// Unrestricted clients can use restricted servers
	client, err := tlsServer.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// certificate authority is not recognized, because
	// the trust has not been established yet
	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// restricted clients can use restricted servers if restrictions
	// exactly match
	identity := TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:k8s"}
	client, err = tlsServer.NewClient(identity)
	require.NoError(t, err)

	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// restricted clients can will be rejected if usage does not match
	identity = TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:extra"}
	client, err = tlsServer.NewClient(identity)
	require.NoError(t, err)

	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	require.Error(t, err)

	// restricted clients can will be rejected, for now if there is any mismatch,
	// including extra usage.
	identity = TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:k8s", "usage:unknown"}
	client, err = tlsServer.NewClient(identity)
	require.NoError(t, err)

	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	require.Error(t, err)
}

// TestRemoteRotation tests remote builtin role
// that attempts certificate authority rotation
func TestRemoteRotation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	var ok bool

	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         t.TempDir(),
		ClusterName: "remote",
		Clock:       testSrv.AuthServer.TestAuthServerConfig.Clock,
	})
	require.NoError(t, err)

	certPool, err := testSrv.CertPool()
	require.NoError(t, err)

	// after trust is established, things are good
	err = testSrv.AuthServer.Trust(ctx, remoteServer, nil)
	require.NoError(t, err)

	remoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleProxy), testSrv.Addr(), certPool)
	require.NoError(t, err)

	remoteAuth, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleAuth), testSrv.Addr(), certPool)
	require.NoError(t, err)

	// remote cluster starts rotation
	gracePeriod := time.Hour
	remoteServer.AuthServer.privateKey, ok = fixtures.PEMBytes["rsa"]
	require.Equal(t, ok, true)
	err = remoteServer.AuthServer.RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// moves to update clients
	err = remoteServer.AuthServer.RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	remoteCA, err := remoteServer.AuthServer.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: remoteServer.ClusterName,
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)

	// remote proxy should be rejected when trying to rotate ca
	// that is not associated with the remote cluster
	clone := remoteCA.Clone()
	clone.SetName(testSrv.ClusterName())
	err = remoteProxy.RotateExternalCertAuthority(ctx, clone)
	require.True(t, trace.IsAccessDenied(err))

	// remote proxy can't upsert the certificate authority,
	// only to rotate it (in remote rotation only certain fields are updated)
	err = remoteProxy.UpsertCertAuthority(ctx, remoteCA)
	require.True(t, trace.IsAccessDenied(err))

	// remote proxy can't read local cert authority with secrets
	_, err = remoteProxy.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, true)
	require.True(t, trace.IsAccessDenied(err))

	// no secrets read is allowed
	_, err = remoteProxy.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)

	// remote auth server will get rejected
	err = remoteAuth.RotateExternalCertAuthority(ctx, remoteCA)
	require.True(t, trace.IsAccessDenied(err))

	// remote proxy should be able to perform remote cert authority
	// rotation
	err = remoteProxy.RotateExternalCertAuthority(ctx, remoteCA)
	require.NoError(t, err)

	// newRemoteProxy should be trusted by the auth server
	newRemoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleProxy), testSrv.Addr(), certPool)
	require.NoError(t, err)

	_, err = newRemoteProxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// old proxy client is still trusted
	_, err = testSrv.CloneClient(t, remoteProxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
}

// TestLocalProxyPermissions tests new local proxy permissions
// as it's now allowed to update host cert authorities of remote clusters
func TestLocalProxyPermissions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         t.TempDir(),
		ClusterName: "remote",
		Clock:       testSrv.AuthServer.TestAuthServerConfig.Clock,
	})
	require.NoError(t, err)

	// after trust is established, things are good
	err = testSrv.AuthServer.Trust(ctx, remoteServer, nil)
	require.NoError(t, err)

	ca, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)

	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// local proxy can't update local cert authorities
	err = proxy.UpsertCertAuthority(ctx, ca)
	require.True(t, trace.IsAccessDenied(err))

	// local proxy is allowed to update host CA of remote cert authorities
	remoteCA, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: remoteServer.ClusterName,
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)

	err = proxy.UpsertCertAuthority(ctx, remoteCA)
	require.NoError(t, err)
}

// TestAutoRotation tests local automatic rotation
func TestAutoRotation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock
	var ok bool

	// create proxy client
	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// starts rotation
	testSrv.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	require.Equal(t, ok, true)
	gracePeriod := time.Hour
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		Mode:        types.RotationModeAuto,
	})
	require.NoError(t, err)

	// advance rotation by clock
	clock.Advance(gracePeriod/3 + time.Minute)
	err = testSrv.Auth().autoRotateCertAuthorities(ctx)
	require.NoError(t, err)

	ca, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)
	require.Equal(t, ca.GetRotation().Phase, types.RotationPhaseUpdateClients)

	// old clients should work
	_, err = testSrv.CloneClient(t, proxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// new clients work as well
	_, err = testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// advance rotation by clock
	clock.Advance((gracePeriod*2)/3 + time.Minute)
	err = testSrv.Auth().autoRotateCertAuthorities(ctx)
	require.NoError(t, err)

	ca, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)
	require.Equal(t, ca.GetRotation().Phase, types.RotationPhaseUpdateServers)

	// old clients should work
	_, err = testSrv.CloneClient(t, proxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// new clients work as well
	newProxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	_, err = newProxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// complete rotation - advance rotation by clock
	clock.Advance(gracePeriod/3 + time.Minute)
	err = testSrv.Auth().autoRotateCertAuthorities(ctx)
	require.NoError(t, err)
	ca, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)
	require.Equal(t, ca.GetRotation().Phase, types.RotationPhaseStandby)
	require.NoError(t, err)

	// old clients should no longer work as soon as backend modification event propagates.
	// new client has to be created here to force re-create the new connection instead of
	// re-using the one from pool this is not going to be a problem in real teleport
	// as it reloads the full server after reload
	require.Eventually(t, func() bool {
		_, err = testSrv.CloneClient(t, proxy).GetNodes(ctx, apidefaults.Namespace)
		// TODO(rosstimothy, espadolini, jakule): figure out how to consistently
		// match a certificate error and not other errors
		return err != nil
	}, time.Second*15, time.Millisecond*200)

	// new clients work
	_, err = testSrv.CloneClient(t, newProxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
}

// TestAutoFallback tests local automatic rotation fallback,
// when user intervenes with rollback and rotation gets switched
// to manual mode
func TestAutoFallback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	var ok bool

	// create proxy client just for test purposes
	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// starts rotation
	testSrv.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	require.Equal(t, ok, true)
	gracePeriod := time.Hour
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		Mode:        types.RotationModeAuto,
	})
	require.NoError(t, err)

	// advance rotation by clock
	clock.Advance(gracePeriod/3 + time.Minute)
	err = testSrv.Auth().autoRotateCertAuthorities(ctx)
	require.NoError(t, err)

	ca, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)
	require.Equal(t, ca.GetRotation().Phase, types.RotationPhaseUpdateClients)
	require.Equal(t, ca.GetRotation().Mode, types.RotationModeAuto)

	// rollback rotation
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseRollback,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	ca, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)
	require.Equal(t, ca.GetRotation().Phase, types.RotationPhaseRollback)
	require.Equal(t, ca.GetRotation().Mode, types.RotationModeManual)
}

// TestManualRotation tests local manual rotation
// that performs full-cycle certificate authority rotation
func TestManualRotation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	var ok bool

	// create proxy client just for test purposes
	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// can't jump to mid-phase
	gracePeriod := time.Hour
	testSrv.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	require.Equal(t, ok, true)
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.True(t, trace.IsBadParameter(err))

	// starts rotation
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// old clients should work
	_, err = testSrv.CloneClient(t, proxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// clients reconnect
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// old clients should work
	_, err = testSrv.CloneClient(t, proxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// new clients work as well
	newProxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	_, err = newProxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// can't jump to standy
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.True(t, trace.IsBadParameter(err))

	// advance rotation:
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// old clients should work
	_, err = testSrv.CloneClient(t, proxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// new clients work as well
	_, err = testSrv.CloneClient(t, newProxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// complete rotation
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// old clients should no longer work as soon as backend modification event propagates.
	// new client has to be created here to force re-create the new connection instead of
	// re-using the one from pool this is not going to be a problem in real teleport
	// as it reloads the full server after reload
	require.Eventually(t, func() bool {
		_, err = testSrv.CloneClient(t, proxy).GetNodes(ctx, apidefaults.Namespace)
		return err != nil
	}, time.Second*15, time.Millisecond*200)

	// new clients work
	_, err = testSrv.CloneClient(t, newProxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
}

// TestRollback tests local manual rotation rollback
func TestRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	var ok bool

	// create proxy client just for test purposes
	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// starts rotation
	gracePeriod := time.Hour
	testSrv.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	require.Equal(t, ok, true)
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// move to update clients phase
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// new clients work
	newProxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	_, err = newProxy.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// advance rotation:
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// rollback rotation
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseRollback,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// new clients work, server still accepts the creds
	// because new clients should re-register and receive new certs
	_, err = testSrv.CloneClient(t, newProxy).GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	// can't jump to other phases
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.True(t, trace.IsBadParameter(err))

	// complete rollback
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// clients with new creds will no longer work as soon as backend modification event propagates.
	require.Eventually(t, func() bool {
		_, err := testSrv.CloneClient(t, newProxy).GetNodes(ctx, apidefaults.Namespace)
		return err != nil
	}, time.Second*15, time.Millisecond*200)

	grpcClientOld := testSrv.CloneClient(t, proxy)
	t.Cleanup(func() {
		require.NoError(t, grpcClientOld.Close())
	})
	// clients with old creds will still work
	_, err = grpcClientOld.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
}

// TestAppTokenRotation checks that JWT tokens can be rotated and tokens can or
// can not be validated at the appropriate phase.
func TestAppTokenRotation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	client, err := testSrv.NewClient(TestBuiltin(types.RoleApp))
	require.NoError(t, err)

	// Create a JWT using the current CA, this will become the "old" CA during
	// rotation.
	oldJWT, err := client.GenerateAppToken(context.Background(),
		types.GenerateAppTokenRequest{
			Username: "foo",
			Roles:    []string{"bar", "baz"},
			Traits: map[string][]string{
				"trait1": {"value1", "value2"},
				"trait2": {"value3", "value4"},
				"trait3": nil,
			},
			URI:     "https://localhost:8080",
			Expires: clock.Now().Add(1 * time.Minute),
		})
	require.NoError(t, err)

	// Check that the "old" CA can be used to verify tokens.
	oldCA, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	require.NoError(t, err)
	require.Len(t, oldCA.GetTrustedJWTKeyPairs(), 1)

	// Verify that the JWT token validates with the JWT authority.
	_, err = verifyJWT(clock, testSrv.ClusterName(), oldCA.GetTrustedJWTKeyPairs(), oldJWT)
	require.NoError(t, err)

	// Start rotation and move to initial phase. A new CA will be added (for
	// verification), but requests will continue to be signed by the old CA.
	gracePeriod := time.Hour
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.JWTSigner,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// At this point in rotation, two JWT key pairs should exist.
	oldCA, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	require.NoError(t, err)
	require.Equal(t, oldCA.GetRotation().Phase, types.RotationPhaseInit)
	require.Len(t, oldCA.GetTrustedJWTKeyPairs(), 2)

	// Verify that the JWT token validates with the JWT authority.
	_, err = verifyJWT(clock, testSrv.ClusterName(), oldCA.GetTrustedJWTKeyPairs(), oldJWT)
	require.NoError(t, err)

	// Move rotation into the update client phase. In this phase, requests will
	// be signed by the new CA, but the old CA will be around to verify requests.
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.JWTSigner,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// New tokens should now fail to validate with the old key.
	newJWT, err := client.GenerateAppToken(ctx,
		types.GenerateAppTokenRequest{
			Username: "foo",
			Roles:    []string{"bar", "baz"},
			Traits: map[string][]string{
				"trait1": {"value1", "value2"},
				"trait2": {"value3", "value4"},
				"trait3": nil,
			},
			URI:     "https://localhost:8080",
			Expires: clock.Now().Add(1 * time.Minute),
		})
	require.NoError(t, err)

	// New tokens will validate with the new key.
	newCA, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	require.NoError(t, err)
	require.Equal(t, newCA.GetRotation().Phase, types.RotationPhaseUpdateClients)
	require.Len(t, newCA.GetTrustedJWTKeyPairs(), 2)

	// Both JWT should now validate.
	_, err = verifyJWT(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT)
	require.NoError(t, err)
	_, err = verifyJWT(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT)
	require.NoError(t, err)

	// Move rotation into update servers phase.
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.JWTSigner,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// At this point only the phase on the CA should have changed.
	newCA, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	require.NoError(t, err)
	require.Equal(t, newCA.GetRotation().Phase, types.RotationPhaseUpdateServers)
	require.Len(t, newCA.GetTrustedJWTKeyPairs(), 2)

	// Both JWT should continue to validate.
	_, err = verifyJWT(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT)
	require.NoError(t, err)
	_, err = verifyJWT(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT)
	require.NoError(t, err)

	// Complete rotation. The old CA will be removed.
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.JWTSigner,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// The new CA should now only have a single key.
	newCA, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	require.NoError(t, err)
	require.Equal(t, newCA.GetRotation().Phase, types.RotationPhaseStandby)
	require.Len(t, newCA.GetTrustedJWTKeyPairs(), 1)

	// Old token should no longer validate.
	_, err = verifyJWT(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT)
	require.Error(t, err)
	_, err = verifyJWT(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT)
	require.NoError(t, err)
}

// TestOIDCIdPTokenRotation checks that OIDC IdP JWT tokens can be rotated and tokens can
// be validated at the appropriate phase.
func TestOIDCIdPTokenRotation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	publicAddress := "https://localhost:8080"

	issuer := "https://my-bucket.s3.amazonaws.com/prefix"

	proxyServer, err := types.NewServer("proxy-hostname", types.KindProxy, types.ServerSpecV2{
		PublicAddrs: []string{publicAddress},
	})
	require.NoError(t, err)

	err = clt.UpsertProxy(ctx, proxyServer)
	require.NoError(t, err)

	integrationName := "my-integration"

	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: integrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN:     "arn:aws:iam::123456789012:role/OpsTeam",
			IssuerS3URI: "s3://my-bucket/prefix",
		},
	)
	require.NoError(t, err)

	_, err = clt.CreateIntegration(ctx, ig)
	require.NoError(t, err)

	client, err := testSrv.NewClient(TestBuiltin(types.RoleDiscovery))
	require.NoError(t, err)

	// Create a JWT using the current CA, this will become the "old" CA during
	// rotation.
	oldJWT, err := client.GenerateAWSOIDCToken(ctx, integrationName)
	require.NoError(t, err)

	// Check that the "old" CA can be used to verify tokens.
	oldCA, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.OIDCIdPCA,
	}, true)
	require.NoError(t, err)
	require.Len(t, oldCA.GetTrustedJWTKeyPairs(), 1)

	// Verify that the JWT token validates with the JWT authority.
	_, err = verifyJWTAWSOIDC(clock, testSrv.ClusterName(), oldCA.GetTrustedJWTKeyPairs(), oldJWT, issuer)
	require.NoError(t, err, clock.Now())

	// Start rotation and move to initial phase. A new CA will be added (for
	// verification), but requests will continue to be signed by the old CA.
	gracePeriod := time.Hour
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.OIDCIdPCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// At this point in rotation, two JWT key pairs should exist.
	oldCA, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.OIDCIdPCA,
	}, true)
	require.NoError(t, err)
	require.Equal(t, oldCA.GetRotation().Phase, types.RotationPhaseInit)
	require.Len(t, oldCA.GetTrustedJWTKeyPairs(), 2)

	// Verify that the JWT token validates with the JWT authority.
	_, err = verifyJWTAWSOIDC(clock, testSrv.ClusterName(), oldCA.GetTrustedJWTKeyPairs(), oldJWT, issuer)
	require.NoError(t, err)

	// Move rotation into the update client phase. In this phase, requests will
	// be signed by the new CA, but the old CA will be around to verify requests.
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.OIDCIdPCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// New tokens should now fail to validate with the old key.
	newJWT, err := client.GenerateAWSOIDCToken(ctx, integrationName)
	require.NoError(t, err)

	// New tokens will validate with the new key.
	newCA, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.OIDCIdPCA,
	}, true)
	require.NoError(t, err)
	require.Equal(t, newCA.GetRotation().Phase, types.RotationPhaseUpdateClients)
	require.Len(t, newCA.GetTrustedJWTKeyPairs(), 2)

	// Both JWT should now validate.
	_, err = verifyJWTAWSOIDC(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT, issuer)
	require.NoError(t, err)
	_, err = verifyJWTAWSOIDC(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT, issuer)
	require.NoError(t, err)

	// Move rotation into update servers phase.
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.OIDCIdPCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// At this point only the phase on the CA should have changed.
	newCA, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.OIDCIdPCA,
	}, true)
	require.NoError(t, err)
	require.Equal(t, newCA.GetRotation().Phase, types.RotationPhaseUpdateServers)
	require.Len(t, newCA.GetTrustedJWTKeyPairs(), 2)

	// Both JWT should continue to validate.
	_, err = verifyJWTAWSOIDC(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT, issuer)
	require.NoError(t, err)
	_, err = verifyJWTAWSOIDC(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT, issuer)
	require.NoError(t, err)

	// Complete rotation. The old CA will be removed.
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.OIDCIdPCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// The new CA should now only have a single key.
	newCA, err = testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.OIDCIdPCA,
	}, true)
	require.NoError(t, err)
	require.Equal(t, newCA.GetRotation().Phase, types.RotationPhaseStandby)
	require.Len(t, newCA.GetTrustedJWTKeyPairs(), 1)

	// Old token should no longer validate.
	_, err = verifyJWTAWSOIDC(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT, issuer)
	require.Error(t, err)
	_, err = verifyJWTAWSOIDC(clock, testSrv.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT, issuer)
	require.NoError(t, err)
}

// TestRemoteUser tests scenario when remote user connects to the local
// auth server and some edge cases.
func TestRemoteUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         t.TempDir(),
		ClusterName: "remote",
		Clock:       clock,
	})
	require.NoError(t, err)

	remoteUser, remoteRole, err := CreateUserAndRole(remoteServer.AuthServer, "remote-user", []string{"remote-role"}, nil)
	require.NoError(t, err)

	certPool, err := testSrv.CertPool()
	require.NoError(t, err)

	remoteClient, err := remoteServer.NewRemoteClient(
		TestUser(remoteUser.GetName()), testSrv.Addr(), certPool)
	require.NoError(t, err)

	// User is not authorized to perform any actions
	// as local cluster does not trust the remote cluster yet
	_, err = remoteClient.GetDomainName(ctx)
	require.True(t, trace.IsConnectionProblem(err))

	// Establish trust, the request will still fail, there is
	// no role mapping set up
	err = testSrv.AuthServer.Trust(ctx, remoteServer, nil)
	require.NoError(t, err)

	// Create fresh client now trust is established
	remoteClient, err = remoteServer.NewRemoteClient(
		TestUser(remoteUser.GetName()), testSrv.Addr(), certPool)
	require.NoError(t, err)
	_, err = remoteClient.GetDomainName(ctx)
	require.True(t, trace.IsAccessDenied(err))

	// Establish trust and map remote role to local admin role
	_, localRole, err := CreateUserAndRole(testSrv.Auth(), "local-user", []string{"local-role"}, nil)
	require.NoError(t, err)

	err = testSrv.AuthServer.Trust(ctx, remoteServer, types.RoleMap{{Remote: remoteRole.GetName(), Local: []string{localRole.GetName()}}})
	require.NoError(t, err)

	_, err = remoteClient.GetDomainName(ctx)
	require.NoError(t, err)
}

// TestNopUser tests user with no permissions except
// the ones that require other authentication methods ("nop" user)
func TestNopUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	client, err := testSrv.NewClient(TestNop())
	require.NoError(t, err)

	// Nop User can get cluster name
	_, err = client.GetDomainName(ctx)
	require.NoError(t, err)

	// But can not get users or nodes
	_, err = client.GetUsers(false)
	require.True(t, trace.IsAccessDenied(err))

	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	require.True(t, trace.IsAccessDenied(err))
}

// TestOwnRole tests that user can read roles assigned to them (used by web UI)
func TestReadOwnRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	user1, userRole, err := CreateUserAndRoleWithoutRoles(clt, "user1", []string{"user1"})
	require.NoError(t, err)

	user2, _, err := CreateUserAndRoleWithoutRoles(clt, "user2", []string{"user2"})
	require.NoError(t, err)

	// user should be able to read their own roles
	userClient, err := testSrv.NewClient(TestUser(user1.GetName()))
	require.NoError(t, err)

	_, err = userClient.GetRole(ctx, userRole.GetName())
	require.NoError(t, err)

	// user2 can't read user1 role
	userClient2, err := testSrv.NewClient(TestIdentity{I: authz.LocalUser{Username: user2.GetName()}})
	require.NoError(t, err)

	_, err = userClient2.GetRole(ctx, userRole.GetName())
	require.True(t, trace.IsAccessDenied(err))
}

func TestGetCurrentUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	srv := newTestTLSServer(t)

	user1, _, err := CreateUserAndRole(srv.Auth(), "user1", []string{"user1"}, nil)
	require.NoError(t, err)

	client1, err := srv.NewClient(TestIdentity{I: authz.LocalUser{Username: user1.GetName()}})
	require.NoError(t, err)

	currentUser, err := client1.GetCurrentUser(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(&types.UserV2{
		Kind:    "user",
		SubKind: "",
		Version: "v2",
		Metadata: types.Metadata{
			Name:        "user1",
			Namespace:   "default",
			Description: "",
			Labels:      nil,
			Expires:     nil,
		},
		Spec: types.UserSpecV2{
			Roles: []string{"user:user1"},
		},
	}, currentUser, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
}

func TestGetCurrentUserRoles(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	user1, user1Role, err := CreateUserAndRole(srv.Auth(), "user1", []string{"user-role"}, nil)
	require.NoError(t, err)

	client1, err := srv.NewClient(TestIdentity{I: authz.LocalUser{Username: user1.GetName()}})
	require.NoError(t, err)

	roles, err := client1.GetCurrentUserRoles(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(roles, []types.Role{user1Role}, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
}

func TestAuthPreferenceSettings(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	suite := &suite.ServicesTestSuite{
		ConfigS: clt,
	}
	suite.AuthPreference(t)
}

func TestTunnelConnectionsCRUD(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
		Clock:     clockwork.NewFakeClock(),
	}
	suite.TunnelConnectionsCRUD(t)
}

func TestRemoteClustersCRUD(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.RemoteClustersCRUD(t)
}

func TestServersCRUD(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.ServerCRUD(t)
}

// TestAppServerCRUD tests CRUD functionality for services.App using an auth client.
func TestAppServerCRUD(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestBuiltin(types.RoleApp))
	require.NoError(t, err)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.AppServerCRUD(t)
}

func TestReverseTunnelsCRUD(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.ReverseTunnelsCRUD(t)
}

func TestUsersCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	usr, err := types.NewUser("user1")
	require.NoError(t, err)
	require.NoError(t, clt.CreateUser(ctx, usr))

	users, err := clt.GetUsers(false)
	require.NoError(t, err)
	require.Equal(t, len(users), 1)
	require.Equal(t, users[0].GetName(), "user1")

	require.NoError(t, clt.DeleteUser(ctx, "user1"))

	users, err = clt.GetUsers(false)
	require.NoError(t, err)
	require.Equal(t, len(users), 0)
}

func TestPasswordGarbage(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	garbage := [][]byte{
		nil,
		make([]byte, defaults.MaxPasswordLength+1),
		make([]byte, defaults.MinPasswordLength-1),
	}
	for _, g := range garbage {
		_, err := testSrv.Auth().checkPassword("user1", g, "123456")
		require.True(t, trace.IsBadParameter(err))
	}
}

func TestPasswordCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	pass := []byte("abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	_, err := testSrv.Auth().checkPassword("user1", pass, "123456")
	require.Error(t, err)

	err = testSrv.Auth().UpsertPassword("user1", pass)
	require.NoError(t, err)

	dev, err := services.NewTOTPDevice("otp", otpSecret, clock.Now())
	require.NoError(t, err)

	err = testSrv.Auth().UpsertMFADevice(ctx, "user1", dev)
	require.NoError(t, err)

	validToken, err := totp.GenerateCode(otpSecret, testSrv.Clock().Now())
	require.NoError(t, err)

	_, err = testSrv.Auth().checkPassword("user1", pass, validToken)
	require.NoError(t, err)
}

func TestOTPCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	user := "user1"
	pass := []byte("abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	// upsert a password and totp secret
	err := testSrv.Auth().UpsertPassword("user1", pass)
	require.NoError(t, err)
	dev, err := services.NewTOTPDevice("otp", otpSecret, clock.Now())
	require.NoError(t, err)

	err = testSrv.Auth().UpsertMFADevice(ctx, user, dev)
	require.NoError(t, err)

	// a completely invalid token should return access denied
	_, err = testSrv.Auth().checkPassword("user1", pass, "123456")
	require.Error(t, err)

	// an invalid token should return access denied
	//
	// this tests makes the token 61 seconds in the future (but from a valid key)
	// even though the validity period is 30 seconds. this is because a token is
	// valid for 30 seconds + 30 second skew before and after for a usability
	// reasons. so a token made between seconds 31 and 60 is still valid, and
	// invalidity starts at 61 seconds in the future.
	invalidToken, err := totp.GenerateCode(otpSecret, testSrv.Clock().Now().Add(61*time.Second))
	require.NoError(t, err)
	_, err = testSrv.Auth().checkPassword("user1", pass, invalidToken)
	require.Error(t, err)

	// a valid token (created right now and from a valid key) should return success
	validToken, err := totp.GenerateCode(otpSecret, testSrv.Clock().Now())
	require.NoError(t, err)

	_, err = testSrv.Auth().checkPassword("user1", pass, validToken)
	require.NoError(t, err)

	// try the same valid token now it should fail because we don't allow re-use of tokens
	_, err = testSrv.Auth().checkPassword("user1", pass, validToken)
	require.Error(t, err)
}

// TestWebSessions tests web sessions flow for web user,
// that logs in, extends web session and tries to perform administrative action
// but fails
func TestWebSessionWithoutAccessRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	user := "user1"
	pass := []byte("abc123")

	_, _, err = CreateUserAndRole(clt, user, []string{user}, nil)
	require.NoError(t, err)

	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	req := authclient.AuthenticateUserRequest{
		Username: user,
		Pass: &authclient.PassCreds{
			Password: pass,
		},
	}
	// authentication attempt fails with no password set up
	_, err = proxy.AuthenticateWebUser(ctx, req)
	require.True(t, trace.IsAccessDenied(err))

	err = testSrv.Auth().UpsertPassword(user, pass)
	require.NoError(t, err)

	// success with password set up
	ws, err := proxy.AuthenticateWebUser(ctx, req)
	require.NoError(t, err)
	require.NotEqual(t, ws, "")

	web, err := testSrv.NewClientFromWebSession(ws)
	require.NoError(t, err)

	_, err = web.GetWebSessionInfo(ctx, user, ws.GetName())
	require.NoError(t, err)

	ns, err := web.ExtendWebSession(ctx, authclient.WebSessionReq{
		User:          user,
		PrevSessionID: ws.GetName(),
	})
	require.NoError(t, err)
	require.NotNil(t, ns)

	// Requesting forbidden action for user fails
	err = web.DeleteUser(ctx, user)
	require.True(t, trace.IsAccessDenied(err))

	err = clt.DeleteWebSession(ctx, user, ws.GetName())
	require.NoError(t, err)

	_, err = web.GetWebSessionInfo(ctx, user, ws.GetName())
	require.Error(t, err)

	_, err = web.ExtendWebSession(ctx, authclient.WebSessionReq{
		User:          user,
		PrevSessionID: ws.GetName(),
	})
	require.Error(t, err)
}

func TestWebSessionMultiAccessRequests(t *testing.T) {
	// Can not use t.Parallel() when changing modules
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// Upsert a node to request access to
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "node1",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.ServerSpecV2{},
	}
	_, err = clt.UpsertNode(ctx, node)
	require.NoError(t, err)
	resourceIDs := []types.ResourceID{{
		Kind:        node.GetKind(),
		Name:        node.GetName(),
		ClusterName: "foobar",
	}}

	// Create user and roles.
	username := "user"
	password := []byte("hunter2")
	baseRoleName := services.RoleNameForUser(username)
	requestableRoleName := "requestable"
	user, err := CreateUserRoleAndRequestable(clt, username, requestableRoleName)
	require.NoError(t, err)
	err = testSrv.Auth().UpsertPassword(username, password)
	require.NoError(t, err)

	// Set search_as_roles, user can request this role only with a resource
	// access request.
	resourceRequestRoleName := "resource-requestable"
	resourceRequestRole := services.RoleForUser(user)
	resourceRequestRole.SetName(resourceRequestRoleName)
	err = clt.UpsertRole(ctx, resourceRequestRole)
	require.NoError(t, err)
	baseRole, err := clt.GetRole(ctx, baseRoleName)
	require.NoError(t, err)
	baseRole.SetSearchAsRoles(types.Allow, []string{resourceRequestRoleName})
	err = clt.UpsertRole(ctx, baseRole)
	require.NoError(t, err)

	// Create approved role request
	roleReq, err := services.NewAccessRequest(username, requestableRoleName)
	require.NoError(t, err)
	roleReq.SetState(types.RequestState_APPROVED)
	roleReq.SetAccessExpiry(clock.Now().Add(8 * time.Hour))
	roleReq, err = clt.CreateAccessRequestV2(ctx, roleReq)
	require.NoError(t, err)

	// Create remote cluster so create access request doesn't err due to non existent cluster
	rc, err := types.NewRemoteCluster("foobar")
	require.NoError(t, err)
	err = testSrv.AuthServer.AuthServer.CreateRemoteCluster(rc)
	require.NoError(t, err)

	// Create approved resource request
	resourceReq, err := services.NewAccessRequestWithResources(username, []string{resourceRequestRoleName}, resourceIDs)
	require.NoError(t, err)
	resourceReq.SetState(types.RequestState_APPROVED)
	resourceReq, err = clt.CreateAccessRequestV2(ctx, resourceReq)
	require.NoError(t, err)

	// Create a web session and client for the user.
	proxyClient, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	baseWebSession, err := proxyClient.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: username,
		Pass: &authclient.PassCreds{
			Password: password,
		},
	})
	require.NoError(t, err)
	proxyClient.Close()
	baseWebClient, err := testSrv.NewClientFromWebSession(baseWebSession)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, baseWebClient.Close()) })

	expectRolesAndResources := func(t *testing.T, sess types.WebSession, expectRoles []string, expectResources []types.ResourceID) {
		sshCert, err := sshutils.ParseCertificate(sess.GetPub())
		require.NoError(t, err)
		gotRoles, err := services.ExtractRolesFromCert(sshCert)
		require.NoError(t, err)
		gotResources, err := services.ExtractAllowedResourcesFromCert(sshCert)
		require.NoError(t, err)
		assert.ElementsMatch(t, expectRoles, gotRoles)
		assert.ElementsMatch(t, expectResources, gotResources)
	}

	type extendSessionFunc func(*testing.T, *authclient.Client, types.WebSession) (*authclient.Client, types.WebSession)
	assumeRequest := func(request types.AccessRequest) extendSessionFunc {
		return func(t *testing.T, clt *authclient.Client, sess types.WebSession) (*authclient.Client, types.WebSession) {
			newSess, err := clt.ExtendWebSession(ctx, authclient.WebSessionReq{
				User:            username,
				PrevSessionID:   sess.GetName(),
				AccessRequestID: request.GetMetadata().Name,
			})
			require.NoError(t, err)
			newClt, err := testSrv.NewClientFromWebSession(newSess)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, newClt.Close()) })
			return newClt, newSess
		}
	}
	failToAssumeRequest := func(request types.AccessRequest) extendSessionFunc {
		return func(t *testing.T, clt *authclient.Client, sess types.WebSession) (*authclient.Client, types.WebSession) {
			_, err := clt.ExtendWebSession(ctx, authclient.WebSessionReq{
				User:            username,
				PrevSessionID:   sess.GetName(),
				AccessRequestID: request.GetMetadata().Name,
			})
			require.Error(t, err)
			return clt, sess
		}
	}
	switchBack := func(t *testing.T, clt *authclient.Client, sess types.WebSession) (*authclient.Client, types.WebSession) {
		newSess, err := clt.ExtendWebSession(ctx, authclient.WebSessionReq{
			User:          username,
			PrevSessionID: sess.GetName(),
			Switchback:    true,
		})
		require.NoError(t, err)
		newClt, err := testSrv.NewClientFromWebSession(newSess)
		require.NoError(t, err)
		return newClt, newSess
	}

	for _, tc := range []struct {
		desc            string
		steps           []extendSessionFunc
		expectRoles     []string
		expectResources []types.ResourceID
	}{
		{
			desc:        "base session",
			expectRoles: []string{baseRoleName},
		},
		{
			desc: "role request",
			steps: []extendSessionFunc{
				assumeRequest(roleReq),
			},
			expectRoles: []string{baseRoleName, requestableRoleName},
		},
		{
			desc: "resource request",
			steps: []extendSessionFunc{
				assumeRequest(resourceReq),
			},
			expectRoles:     []string{baseRoleName, resourceRequestRoleName},
			expectResources: resourceIDs,
		},
		{
			desc: "role then resource",
			steps: []extendSessionFunc{
				assumeRequest(roleReq),
				assumeRequest(resourceReq),
			},
			expectRoles:     []string{baseRoleName, requestableRoleName, resourceRequestRoleName},
			expectResources: resourceIDs,
		},
		{
			desc: "resource then role",
			steps: []extendSessionFunc{
				assumeRequest(resourceReq),
				assumeRequest(roleReq),
			},
			expectRoles:     []string{baseRoleName, requestableRoleName, resourceRequestRoleName},
			expectResources: resourceIDs,
		},
		{
			desc: "duplicates",
			steps: []extendSessionFunc{
				assumeRequest(resourceReq),
				assumeRequest(roleReq),
				// Cannot combine resource requests, this also blocks assuming
				// the same one twice.
				failToAssumeRequest(resourceReq),
				assumeRequest(roleReq),
			},
			expectRoles:     []string{baseRoleName, requestableRoleName, resourceRequestRoleName},
			expectResources: resourceIDs,
		},
		{
			desc: "switch back",
			steps: []extendSessionFunc{
				assumeRequest(roleReq),
				assumeRequest(resourceReq),
				switchBack,
			},
			expectRoles: []string{baseRoleName},
		},
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			clt, sess := baseWebClient, baseWebSession
			for _, extendSession := range tc.steps {
				clt, sess = extendSession(t, clt, sess)
			}
			expectRolesAndResources(t, sess, tc.expectRoles, tc.expectResources)
		})
	}
}

func TestWebSessionWithApprovedAccessRequestAndSwitchback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	user := "user2"
	pass := []byte("abc123")

	newUser, err := CreateUserRoleAndRequestable(clt, user, "test-request-role")
	require.NoError(t, err)
	require.Len(t, newUser.GetRoles(), 1)
	require.Empty(t, cmp.Diff(newUser.GetRoles(), []string{"user:user2"}))

	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// Create a user to create a web session for.
	req := authclient.AuthenticateUserRequest{
		Username: user,
		Pass: &authclient.PassCreds{
			Password: pass,
		},
	}

	err = testSrv.Auth().UpsertPassword(user, pass)
	require.NoError(t, err)

	ws, err := proxy.AuthenticateWebUser(ctx, req)
	require.NoError(t, err)

	web, err := testSrv.NewClientFromWebSession(ws)
	require.NoError(t, err)

	initialRole := newUser.GetRoles()[0]
	initialSession, err := web.GetWebSessionInfo(ctx, user, ws.GetName())
	require.NoError(t, err)

	// Create a approved access request.
	accessReq, err := services.NewAccessRequest(user, []string{"test-request-role"}...)
	require.NoError(t, err)

	// Set a lesser expiry date, to test switching back to default expiration later.
	accessReq.SetAccessExpiry(clock.Now().Add(time.Minute * 10))
	accessReq.SetState(types.RequestState_APPROVED)

	accessReq, err = clt.CreateAccessRequestV2(ctx, accessReq)
	require.NoError(t, err)

	sess1, err := web.ExtendWebSession(ctx, authclient.WebSessionReq{
		User:            user,
		PrevSessionID:   ws.GetName(),
		AccessRequestID: accessReq.GetMetadata().Name,
	})
	require.NoError(t, err)
	require.WithinDuration(t, clock.Now().Add(time.Minute*10), sess1.Expiry(), time.Second)
	require.WithinDuration(t, sess1.GetLoginTime(), initialSession.GetLoginTime(), time.Second)

	sshcert, err := sshutils.ParseCertificate(sess1.GetPub())
	require.NoError(t, err)

	// Roles extracted from cert should contain the initial role and the role assigned with access request.
	roles, err := services.ExtractRolesFromCert(sshcert)
	require.NoError(t, err)
	require.Len(t, roles, 2)

	mappedRole := map[string]string{
		roles[0]: "",
		roles[1]: "",
	}

	_, hasRole := mappedRole[initialRole]
	require.Equal(t, hasRole, true)

	_, hasRole = mappedRole["test-request-role"]
	require.Equal(t, hasRole, true)

	// certRequests extracts the active requests from a PEM encoded TLS cert.
	certRequests := func(tlsCert []byte) []string {
		cert, err := tlsca.ParseCertificatePEM(tlsCert)
		require.NoError(t, err)

		identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		require.NoError(t, err)

		return identity.ActiveRequests
	}

	require.Empty(t, cmp.Diff(certRequests(sess1.GetTLSCert()), []string{accessReq.GetName()}))

	// Test switch back to default role and expiry.
	sess2, err := web.ExtendWebSession(ctx, authclient.WebSessionReq{
		User:          user,
		PrevSessionID: ws.GetName(),
		Switchback:    true,
	})
	require.NoError(t, err)
	require.Equal(t, sess2.GetExpiryTime(), initialSession.GetExpiryTime())
	require.Equal(t, sess2.GetLoginTime(), initialSession.GetLoginTime())

	sshcert, err = sshutils.ParseCertificate(sess2.GetPub())
	require.NoError(t, err)

	roles, err = services.ExtractRolesFromCert(sshcert)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(roles, []string{initialRole}))

	require.Len(t, certRequests(sess2.GetTLSCert()), 0)
}

func TestExtendWebSessionWithReloadUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	user := "user2"
	pass := []byte("abc123")

	newUser, _, err := CreateUserAndRole(clt, user, nil, nil)
	require.NoError(t, err)
	require.Empty(t, newUser.GetTraits())

	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// Create user authn creds and web session.
	req := authclient.AuthenticateUserRequest{
		Username: user,
		Pass: &authclient.PassCreds{
			Password: pass,
		},
	}
	err = testSrv.Auth().UpsertPassword(user, pass)
	require.NoError(t, err)
	ws, err := proxy.AuthenticateWebUser(ctx, req)
	require.NoError(t, err)
	web, err := testSrv.NewClientFromWebSession(ws)
	require.NoError(t, err)

	// Update some traits and roles.
	newRoleName := "new-role"
	newUser.SetLogins([]string{"apple", "banana"})
	newUser.SetDatabaseUsers([]string{"llama", "alpaca"})
	_, err = CreateRole(ctx, clt, newRoleName, types.RoleSpecV6{})
	require.NoError(t, err)
	newUser.AddRole(newRoleName)
	require.NoError(t, clt.UpdateUser(ctx, newUser))

	// Renew session with the updated traits.
	sess1, err := web.ExtendWebSession(ctx, authclient.WebSessionReq{
		User:          user,
		PrevSessionID: ws.GetName(),
		ReloadUser:    true,
	})
	require.NoError(t, err)

	// Check traits has been updated to latest.
	sshcert, err := sshutils.ParseCertificate(sess1.GetPub())
	require.NoError(t, err)
	traits, err := services.ExtractTraitsFromCert(sshcert)
	require.NoError(t, err)
	roles, err := services.ExtractRolesFromCert(sshcert)
	require.NoError(t, err)
	require.Equal(t, traits[constants.TraitLogins], []string{"apple", "banana"})
	require.Equal(t, traits[constants.TraitDBUsers], []string{"llama", "alpaca"})
	require.Contains(t, roles, newRoleName)
}

func TestExtendWebSessionWithMaxDuration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	adminClient, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	const user = "user2"
	const testRequestRole = "test-request-role"
	pass := []byte("abc123")

	newUser, err := CreateUserRoleAndRequestable(adminClient, user, testRequestRole)
	require.NoError(t, err)
	require.Len(t, newUser.GetRoles(), 1)

	require.Len(t, newUser.GetRoles(), 1)
	require.Empty(t, cmp.Diff(newUser.GetRoles(), []string{"user:user2"}))

	proxyRoleClient, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// Create a user to create a web session for.
	req := authclient.AuthenticateUserRequest{
		Username: user,
		Pass: &authclient.PassCreds{
			Password: pass,
		},
	}

	err = testSrv.Auth().UpsertPassword(user, pass)
	require.NoError(t, err)

	webSession, err := proxyRoleClient.AuthenticateWebUser(ctx, req)
	require.NoError(t, err)

	userClient, err := testSrv.NewClientFromWebSession(webSession)
	require.NoError(t, err)

	testCases := []struct {
		desc            string
		maxDurationRole time.Duration
		expectedExpiry  time.Duration
	}{
		{
			desc:            "default",
			maxDurationRole: 0,
			expectedExpiry:  apidefaults.CertDuration,
		},
		{
			desc:            "max duration is set",
			maxDurationRole: 5 * time.Hour,
			expectedExpiry:  5 * time.Hour,
		},
		{
			desc:            "max duration greater than default",
			maxDurationRole: 24 * time.Hour,
			expectedExpiry:  apidefaults.CertDuration,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			requestableRole, err := adminClient.GetRole(ctx, newUser.GetRoles()[0])
			require.NoError(t, err)

			// Set max duration on the role.
			requestableRole.SetAccessRequestConditions(types.Allow, types.AccessRequestConditions{
				Roles:       []string{testRequestRole},
				MaxDuration: types.Duration(tc.maxDurationRole),
			})
			err = adminClient.UpsertRole(ctx, requestableRole)
			require.NoError(t, err)

			// Create an approved access request.
			accessReq, err := services.NewAccessRequest(user, []string{testRequestRole}...)
			require.NoError(t, err)

			// Set max duration higher than the role max duration. It will be capped.
			accessReq.SetMaxDuration(clock.Now().Add(48 * time.Hour))
			err = accessReq.SetState(types.RequestState_APPROVED)
			require.NoError(t, err)

			accessReq, err = adminClient.CreateAccessRequestV2(ctx, accessReq)
			require.NoError(t, err)

			sess1, err := userClient.ExtendWebSession(ctx, authclient.WebSessionReq{
				User:            user,
				PrevSessionID:   webSession.GetName(),
				AccessRequestID: accessReq.GetMetadata().Name,
			})
			require.NoError(t, err)

			// Check the expiry is capped to the max allowed duration.
			require.WithinDuration(t, clock.Now().Add(tc.expectedExpiry), sess1.Expiry(), time.Second)
		})
	}
}

// TestGetCertAuthority tests certificate authority permissions
func TestGetCertAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	// generate server keys for node
	nodeClt, err := testSrv.NewClient(TestIdentity{I: authz.BuiltinRole{Username: "00000000-0000-0000-0000-000000000000", Role: types.RoleNode}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeClt.Close()) })

	hostCAID := types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}

	// node is authorized to fetch CA without secrets
	ca, err := nodeClt.GetCertAuthority(ctx, hostCAID, false)
	require.NoError(t, err)
	for _, keyPair := range ca.GetActiveKeys().TLS {
		require.Nil(t, keyPair.Key)
	}
	for _, keyPair := range ca.GetActiveKeys().SSH {
		require.Nil(t, keyPair.PrivateKey)
	}

	// node is not authorized to fetch CA with secrets
	_, err = nodeClt.GetCertAuthority(ctx, hostCAID, true)
	require.True(t, trace.IsAccessDenied(err))

	// generate server keys for proxy
	proxyClt, err := testSrv.NewClient(TestIdentity{
		I: authz.BuiltinRole{
			Username: "00000000-0000-0000-0000-000000000001",
			Role:     types.RoleProxy,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyClt.Close()) })

	// proxy can't fetch the host CA with secrets
	_, err = proxyClt.GetCertAuthority(ctx, hostCAID, true)
	require.True(t, trace.IsAccessDenied(err))

	// proxy can't fetch SAML IdP CA with secrets
	_, err = proxyClt.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.SAMLIDPCA,
	}, true)
	require.True(t, trace.IsAccessDenied(err))

	// proxy can't fetch anything else with secrets
	_, err = proxyClt.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.DatabaseCA,
	}, true)
	require.True(t, trace.IsAccessDenied(err))
	_, err = proxyClt.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.DatabaseClientCA,
	}, true)
	require.True(t, trace.IsAccessDenied(err))

	_, err = proxyClt.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.OIDCIdPCA,
	}, true)
	require.True(t, trace.IsAccessDenied(err))

	// non-admin users are not allowed to get access to private key material
	user, err := types.NewUser("bob")
	require.NoError(t, err)

	role := services.RoleForUser(user)
	role.SetLogins(types.Allow, []string{user.GetName()})
	err = testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	user.AddRole(role.GetName())
	err = testSrv.Auth().UpsertUser(user)
	require.NoError(t, err)

	userClt, err := testSrv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)
	defer userClt.Close()

	// user is authorized to fetch CA without secrets
	_, err = userClt.GetCertAuthority(ctx, hostCAID, false)
	require.NoError(t, err)

	// user is not authorized to fetch CA with secrets
	_, err = userClt.GetCertAuthority(ctx, hostCAID, true)
	require.True(t, trace.IsAccessDenied(err))

	// user gets a not found message if a CA doesn't exist
	require.NoError(t, testSrv.Auth().DeleteCertAuthority(ctx, hostCAID))
	_, err = userClt.GetCertAuthority(ctx, hostCAID, false)
	require.True(t, trace.IsNotFound(err))

	// user is not authorized to fetch CA with secrets, even if CA doesn't exist
	_, err = userClt.GetCertAuthority(ctx, hostCAID, true)
	require.True(t, trace.IsAccessDenied(err))
}

func TestPluginData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	// make sure we can parse the private and public key
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	require.NoError(t, err)

	_, err = tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	require.NoError(t, err)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)

	user := "user1"
	role := "some-role"
	_, err = CreateUserRoleAndRequestable(testSrv.Auth(), user, role)
	require.NoError(t, err)

	testUser := TestUser(user)
	testUser.TTL = time.Hour
	userClient, err := testSrv.NewClient(testUser)
	require.NoError(t, err)

	plugin := "my-plugin"
	_, err = CreateAccessPluginUser(ctx, testSrv.Auth(), plugin)
	require.NoError(t, err)

	pluginUser := TestUser(plugin)
	pluginUser.TTL = time.Hour
	pluginClient, err := testSrv.NewClient(pluginUser)
	require.NoError(t, err)

	req, err := services.NewAccessRequest(user, role)
	require.NoError(t, err)

	req, err = userClient.CreateAccessRequestV2(ctx, req)
	require.NoError(t, err)

	err = pluginClient.UpdatePluginData(ctx, types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: req.GetName(),
		Plugin:   plugin,
		Set: map[string]string{
			"foo": "bar",
		},
	})
	require.NoError(t, err)

	data, err := pluginClient.GetPluginData(ctx, types.PluginDataFilter{
		Kind:     types.KindAccessRequest,
		Resource: req.GetName(),
	})
	require.NoError(t, err)
	require.Equal(t, len(data), 1)

	entry, ok := data[0].Entries()[plugin]
	require.Equal(t, ok, true)
	require.Empty(t, cmp.Diff(entry.Data, map[string]string{"foo": "bar"}))

	err = pluginClient.UpdatePluginData(ctx, types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: req.GetName(),
		Plugin:   plugin,
		Set: map[string]string{
			"foo":  "",
			"spam": "eggs",
		},
		Expect: map[string]string{
			"foo": "bar",
		},
	})
	require.NoError(t, err)

	data, err = pluginClient.GetPluginData(ctx, types.PluginDataFilter{
		Kind:     types.KindAccessRequest,
		Resource: req.GetName(),
	})
	require.NoError(t, err)
	require.Equal(t, len(data), 1)

	entry, ok = data[0].Entries()[plugin]
	require.Equal(t, ok, true)
	require.Empty(t, cmp.Diff(entry.Data, map[string]string{"spam": "eggs"}))
}

// TestGenerateCerts tests edge cases around authorization of
// certificate generation for servers and users
func TestGenerateCerts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv := newTestTLSServer(t)
	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	clock := srv.Auth().GetClock()

	// make sure we can parse the private and public key
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	require.NoError(t, err)

	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	require.NoError(t, err)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)

	// generate server keys for node
	hostID := "00000000-0000-0000-0000-000000000000"
	hostClient, err := srv.NewClient(TestIdentity{I: authz.BuiltinRole{Username: hostID, Role: types.RoleNode}})
	require.NoError(t, err)

	certs, err := hostClient.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               hostID,
			NodeName:             srv.AuthServer.ClusterName,
			Role:                 types.RoleNode,
			AdditionalPrincipals: []string{"example.com"},
			PublicSSHKey:         pub,
			PublicTLSKey:         pubTLS,
		})
	require.NoError(t, err)

	hostCert, err := sshutils.ParseCertificate(certs.SSH)
	require.NoError(t, err)
	require.Contains(t, hostCert.ValidPrincipals, "example.com")

	// sign server public keys for node
	hostID = "00000000-0000-0000-0000-000000000000"
	hostClient, err = srv.NewClient(TestIdentity{I: authz.BuiltinRole{Username: hostID, Role: types.RoleNode}})
	require.NoError(t, err)

	certs, err = hostClient.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               hostID,
			NodeName:             srv.AuthServer.ClusterName,
			Role:                 types.RoleNode,
			AdditionalPrincipals: []string{"example.com"},
			PublicSSHKey:         pub,
			PublicTLSKey:         pubTLS,
		})
	require.NoError(t, err)

	hostCert, err = sshutils.ParseCertificate(certs.SSH)
	require.NoError(t, err)
	require.Contains(t, hostCert.ValidPrincipals, "example.com")

	t.Run("HostClients", func(t *testing.T) {
		// attempt to elevate privileges by getting admin role in the certificate
		_, err = hostClient.GenerateHostCerts(context.Background(),
			&proto.HostCertsRequest{
				HostID:       hostID,
				NodeName:     srv.AuthServer.ClusterName,
				Role:         types.RoleAdmin,
				PublicSSHKey: pub,
				PublicTLSKey: pubTLS,
			})
		require.True(t, trace.IsAccessDenied(err))

		// attempt to get certificate for different host id
		_, err = hostClient.GenerateHostCerts(context.Background(),
			&proto.HostCertsRequest{
				HostID:       "some-other-host-id",
				NodeName:     srv.AuthServer.ClusterName,
				Role:         types.RoleNode,
				PublicSSHKey: pub,
				PublicTLSKey: pubTLS,
			})
		require.True(t, trace.IsAccessDenied(err))
	})

	user1, userRole, err := CreateUserAndRole(srv.Auth(), "user1", []string{"user1"}, nil)
	require.NoError(t, err)

	user2, userRole2, err := CreateUserAndRole(srv.Auth(), "user2", []string{"user2"}, nil)
	require.NoError(t, err)

	t.Run("Nop", func(t *testing.T) {
		// unauthenticated client should NOT be able to generate a user cert without auth
		nopClient, err := srv.NewClient(TestNop())
		require.NoError(t, err)

		_, err = nopClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   clock.Now().Add(time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), err.Error())
	})

	testUser2 := TestUser(user2.GetName())
	testUser2.TTL = time.Hour
	userClient2, err := srv.NewClient(testUser2)
	require.NoError(t, err)

	t.Run("ImpersonateDeny", func(t *testing.T) {
		// User can't generate certificates for another user by default
		_, err = userClient2.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   clock.Now().Add(time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	parseCert := func(sshCert []byte) (*ssh.Certificate, time.Duration) {
		parsedCert, err := sshutils.ParseCertificate(sshCert)
		require.NoError(t, err)
		validBefore := time.Unix(int64(parsedCert.ValidBefore), 0)
		return parsedCert, validBefore.Sub(clock.Now())
	}

	t.Run("ImpersonateAllow", func(t *testing.T) {
		// Super impersonator impersonate anyone and login as root
		maxSessionTTL := 300 * time.Hour
		authPref, err := srv.Auth().GetAuthPreference(ctx)
		require.NoError(t, err)
		authPref.SetDefaultSessionTTL(types.Duration(maxSessionTTL))
		srv.Auth().SetAuthPreference(ctx, authPref)
		superImpersonatorRole, err := types.NewRole("superimpersonator", types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.Duration(maxSessionTTL),
			},
			Allow: types.RoleConditions{
				Logins: []string{"root"},
				Impersonate: &types.ImpersonateConditions{
					Users: []string{types.Wildcard},
					Roles: []string{types.Wildcard},
				},
				Rules: []types.Rule{},
			},
		})
		require.NoError(t, err)
		superImpersonator, err := CreateUser(srv.Auth(), "superimpersonator", superImpersonatorRole)
		require.NoError(t, err)

		// Impersonator can generate certificates for super impersonator
		role, err := types.NewRole("impersonate", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{superImpersonator.GetName()},
				Impersonate: &types.ImpersonateConditions{
					Users: []string{superImpersonator.GetName()},
					Roles: []string{superImpersonatorRole.GetName()},
				},
			},
		})
		require.NoError(t, err)
		impersonator, err := CreateUser(srv.Auth(), "impersonator", role)
		require.NoError(t, err)

		iUser := TestUser(impersonator.GetName())
		iUser.TTL = time.Hour
		iClient, err := srv.NewClient(iUser)
		require.NoError(t, err)

		// can impersonate super impersonator and request certs
		// longer than their own TTL, but not exceeding super impersonator's max session ttl
		userCerts, err := iClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  superImpersonator.GetName(),
			Expires:   clock.Now().Add(1000 * time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.NoError(t, err)

		_, diff := parseCert(userCerts.SSH)
		require.LessOrEqual(t, diff, maxSessionTTL)

		tlsCert, err := tlsca.ParseCertificatePEM(userCerts.TLS)
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)

		// Because the original request has maxed out the possible max
		// session TTL, it will be adjusted to exactly the value (within rounding errors)
		require.WithinDuration(t, clock.Now().Add(maxSessionTTL), identity.Expires, time.Second)
		require.Equal(t, impersonator.GetName(), identity.Impersonator)
		require.Equal(t, superImpersonator.GetName(), identity.Username)

		// impersonator can't impersonate user1
		_, err = iClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   clock.Now().Add(time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "trace.IsAccessDenied failed: err=%v (%T)", err, trace.Unwrap(err))

		_, privateKeyPEM, err := utils.MarshalPrivateKey(privateKey.(crypto.Signer))
		require.NoError(t, err)

		clientCert, err := tls.X509KeyPair(userCerts.TLS, privateKeyPEM)
		require.NoError(t, err)

		// client that uses impersonated certificate can't impersonate other users
		// although super impersonator's roles allow it
		impersonatedClient := srv.NewClientWithCert(clientCert)
		_, err = impersonatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   clock.Now().Add(time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "trace.IsAccessDenied failed: err=%v (%T)", err, trace.Unwrap(err))
		require.Contains(t, err.Error(), "impersonated user can not impersonate anyone else")

		// but can renew their own cert, for example set route to cluster
		rc, err := types.NewRemoteCluster("cluster-remote")
		require.NoError(t, err)
		err = srv.Auth().CreateRemoteCluster(rc)
		require.NoError(t, err)

		userCerts, err = impersonatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey:      pub,
			Username:       superImpersonator.GetName(),
			Expires:        clock.Now().Add(time.Hour).UTC(),
			Format:         constants.CertificateFormatStandard,
			RouteToCluster: rc.GetName(),
		})
		require.NoError(t, err)
		// Make sure impersonator was not lost in the renewed cert
		tlsCert, err = tlsca.ParseCertificatePEM(userCerts.TLS)
		require.NoError(t, err)
		identity, err = tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)
		require.WithinDuration(t, identity.Expires, clock.Now().Add(time.Hour), time.Second)
		require.Equal(t, impersonator.GetName(), identity.Impersonator)
		require.Equal(t, superImpersonator.GetName(), identity.Username)
	})

	t.Run("Renew", func(t *testing.T) {
		testUser2 := TestUser(user2.GetName())
		testUser2.TTL = time.Hour
		userClient2, err := srv.NewClient(testUser2)
		require.NoError(t, err)

		rc1, err := types.NewRemoteCluster("cluster1")
		require.NoError(t, err)
		err = srv.Auth().CreateRemoteCluster(rc1)
		require.NoError(t, err)

		// User can renew their certificates, however the TTL will be limited
		// to the TTL of their session for both SSH and x509 certs and
		// that route to cluster will be encoded in the cert metadata
		userCerts, err := userClient2.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey:      pub,
			Username:       user2.GetName(),
			Expires:        clock.Now().Add(100 * time.Hour).UTC(),
			Format:         constants.CertificateFormatStandard,
			RouteToCluster: rc1.GetName(),
		})
		require.NoError(t, err)

		_, diff := parseCert(userCerts.SSH)
		require.LessOrEqual(t, diff, testUser2.TTL)

		tlsCert, err := tlsca.ParseCertificatePEM(userCerts.TLS)
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)
		require.WithinDuration(t, clock.Now().Add(testUser2.TTL), identity.Expires, time.Second)
		require.Equal(t, identity.RouteToCluster, rc1.GetName())
	})

	t.Run("Admin", func(t *testing.T) {
		// Admin should be allowed to generate certs with TTL longer than max.
		adminClient, err := srv.NewClient(TestAdmin())
		require.NoError(t, err)

		userCerts, err := adminClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   clock.Now().Add(40 * time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.NoError(t, err)

		parsedCert, diff := parseCert(userCerts.SSH)
		require.Less(t, apidefaults.MaxCertDuration, diff)

		// user should have agent forwarding (default setting)
		require.Contains(t, parsedCert.Extensions, teleport.CertExtensionPermitAgentForwarding)

		// user should not have X11 forwarding (default setting)
		require.NotContains(t, parsedCert.Extensions, teleport.CertExtensionPermitX11Forwarding)

		// now update role to permit agent and X11 forwarding
		roleOptions := userRole.GetOptions()
		roleOptions.ForwardAgent = types.NewBool(true)
		roleOptions.PermitX11Forwarding = types.NewBool(true)
		userRole.SetOptions(roleOptions)
		err = srv.Auth().UpsertRole(ctx, userRole)
		require.NoError(t, err)

		userCerts, err = adminClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   clock.Now().Add(1 * time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.NoError(t, err)
		parsedCert, _ = parseCert(userCerts.SSH)

		// user should get agent forwarding
		require.Contains(t, parsedCert.Extensions, teleport.CertExtensionPermitAgentForwarding)

		// user should get X11 forwarding
		require.Contains(t, parsedCert.Extensions, teleport.CertExtensionPermitX11Forwarding)

		// apply HTTP Auth to generate user cert:
		userCerts, err = adminClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   clock.Now().Add(time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.NoError(t, err)

		_, _, _, _, err = ssh.ParseAuthorizedKey(userCerts.SSH)
		require.NoError(t, err)
	})

	t.Run("DenyLeaf", func(t *testing.T) {
		// User can't generate certificates for an unknown leaf cluster.
		_, err = userClient2.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey:      pub,
			Username:       user2.GetName(),
			Expires:        clock.Now().Add(100 * time.Hour).UTC(),
			Format:         constants.CertificateFormatStandard,
			RouteToCluster: "unknown_cluster",
		})
		require.Error(t, err)

		rc2, err := types.NewRemoteCluster("cluster2")
		require.NoError(t, err)
		meta := rc2.GetMetadata()
		meta.Labels = map[string]string{"env": "prod"}
		rc2.SetMetadata(meta)
		err = srv.Auth().CreateRemoteCluster(rc2)
		require.NoError(t, err)

		// User can't generate certificates for leaf cluster they don't have access
		// to due to labels.
		_, err = userClient2.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey:      pub,
			Username:       user2.GetName(),
			Expires:        clock.Now().Add(100 * time.Hour).UTC(),
			Format:         constants.CertificateFormatStandard,
			RouteToCluster: rc2.GetName(),
		})
		require.Error(t, err)

		userRole2.SetClusterLabels(types.Allow, types.Labels{"env": apiutils.Strings{"prod"}})
		err = srv.Auth().UpsertRole(ctx, userRole2)
		require.NoError(t, err)

		// User can generate certificates for leaf cluster they do have access to.
		userCerts, err := userClient2.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey:      pub,
			Username:       user2.GetName(),
			Expires:        clock.Now().Add(100 * time.Hour).UTC(),
			Format:         constants.CertificateFormatStandard,
			RouteToCluster: rc2.GetName(),
		})
		require.NoError(t, err)

		tlsCert, err := tlsca.ParseCertificatePEM(userCerts.TLS)
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)
		require.Equal(t, identity.RouteToCluster, rc2.GetName())
	})
}

// TestGenerateAppToken checks the identity of the caller and makes sure only
// certain roles can request JWT tokens.
func TestGenerateAppToken(t *testing.T) {
	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	authClient, err := testSrv.NewClient(TestBuiltin(types.RoleAdmin))
	require.NoError(t, err)

	ca, err := authClient.GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: testSrv.ClusterName(),
	}, true)
	require.NoError(t, err)

	signer, err := testSrv.AuthServer.AuthServer.GetKeyStore().GetJWTSigner(ctx, ca)
	require.NoError(t, err)
	key, err := services.GetJWTSigner(signer, ca.GetClusterName(), clock)
	require.NoError(t, err)

	tests := []struct {
		inMachineRole types.SystemRole
		inComment     string
		outError      bool
	}{
		{
			inMachineRole: types.RoleNode,
			inComment:     "nodes should not have the ability to generate tokens",
			outError:      true,
		},
		{
			inMachineRole: types.RoleProxy,
			inComment:     "proxies should not have the ability to generate tokens",
			outError:      true,
		},
		{
			inMachineRole: types.RoleApp,
			inComment:     "only apps should have the ability to generate tokens",
			outError:      false,
		},
	}
	for _, ts := range tests {
		client, err := testSrv.NewClient(TestBuiltin(ts.inMachineRole))
		require.NoError(t, err, ts.inComment)

		token, err := client.GenerateAppToken(
			context.Background(),
			types.GenerateAppTokenRequest{
				Username: "foo@example.com",
				Roles:    []string{"bar", "baz"},
				Traits: wrappers.Traits{
					"trait1": {"value1", "value2"},
					"trait2": {"value3", "value4"},
					"trait3": nil,
				},
				URI:     "https://localhost:8080",
				Expires: clock.Now().Add(1 * time.Minute),
			})
		require.Equal(t, err != nil, ts.outError, ts.inComment)
		if !ts.outError {
			claims, err := key.Verify(jwt.VerifyParams{
				Username: "foo@example.com",
				RawToken: token,
				URI:      "https://localhost:8080",
			})
			require.NoError(t, err, ts.inComment)
			require.Equal(t, claims.Username, "foo@example.com", ts.inComment)
			require.Empty(t, cmp.Diff(claims.Roles, []string{"bar", "baz"}), ts.inComment)
			require.Empty(t, cmp.Diff(claims.Traits, wrappers.Traits{
				"trait1": {"value1", "value2"},
				"trait2": {"value3", "value4"},
			}), ts.inComment)
		}
	}
}

// TestCertificateFormat makes sure that certificates are generated with the
// correct format.
func TestCertificateFormat(t *testing.T) {
	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	require.NoError(t, err)
	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)

	// use admin client to create user and role
	user, userRole, err := CreateUserAndRole(testSrv.Auth(), "user", []string{"user"}, nil)
	require.NoError(t, err)

	pass := []byte("very secure password")
	err = testSrv.Auth().UpsertPassword(user.GetName(), pass)
	require.NoError(t, err)

	tests := []struct {
		inRoleCertificateFormat   string
		inClientCertificateFormat string
		outCertContainsRole       bool
	}{
		// 0 - take whatever the role has
		{
			teleport.CertificateFormatOldSSH,
			teleport.CertificateFormatUnspecified,
			false,
		},
		// 1 - override the role
		{
			teleport.CertificateFormatOldSSH,
			constants.CertificateFormatStandard,
			true,
		},
	}

	for _, ts := range tests {
		roleOptions := userRole.GetOptions()
		roleOptions.CertificateFormat = ts.inRoleCertificateFormat
		userRole.SetOptions(roleOptions)
		err := testSrv.Auth().UpsertRole(ctx, userRole)
		require.NoError(t, err)

		proxyClient, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
		require.NoError(t, err)

		// authentication attempt fails with password auth only
		re, err := proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
			AuthenticateUserRequest: authclient.AuthenticateUserRequest{
				Username: user.GetName(),
				Pass: &authclient.PassCreds{
					Password: pass,
				},
				PublicKey: pub,
			},
			CompatibilityMode: ts.inClientCertificateFormat,
			TTL:               apidefaults.CertDuration,
		})
		require.NoError(t, err)

		parsedCert, err := sshutils.ParseCertificate(re.Cert)
		require.NoError(t, err)

		_, ok := parsedCert.Extensions[teleport.CertExtensionTeleportRoles]
		require.Equal(t, ok, ts.outCertContainsRole)
	}
}

// TestClusterConfigContext checks that the cluster configuration gets passed
// along in the context and permissions get updated accordingly.
func TestClusterConfigContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	_, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	// try and generate a host cert, this should succeed because although
	// we are recording at the nodes not at the proxy, the proxy may
	// need to generate host certs if a client wants to connect to an
	// agentless node
	_, err = proxy.GenerateHostCert(ctx, pub,
		"a", "b", nil,
		"localhost", types.RoleProxy, 0)
	require.NoError(t, err)

	// update cluster config to record at the proxy
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	require.NoError(t, err)
	err = testSrv.Auth().SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// try and generate a host cert
	_, err = proxy.GenerateHostCert(ctx, pub,
		"a", "b", nil,
		"localhost", types.RoleProxy, 0)
	require.NoError(t, err)
}

// TestAuthenticateWebUserOTP tests web authentication flow for password + OTP
func TestAuthenticateWebUserOTP(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	user := "ws-test"
	pass := []byte("ws-abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	_, _, err = CreateUserAndRole(clt, user, []string{user}, nil)
	require.NoError(t, err)

	err = testSrv.Auth().UpsertPassword(user, pass)
	require.NoError(t, err)

	dev, err := services.NewTOTPDevice("otp", otpSecret, clock.Now())
	require.NoError(t, err)
	err = testSrv.Auth().UpsertMFADevice(ctx, user, dev)
	require.NoError(t, err)

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, clock.Now())
	require.NoError(t, err)

	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)
	err = testSrv.Auth().SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	// authentication attempt fails with wrong password
	_, err = proxy.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: user,
		OTP:      &authclient.OTPCreds{Password: []byte("wrong123"), Token: validToken},
	})
	require.True(t, trace.IsAccessDenied(err))

	// authentication attempt fails with wrong otp
	_, err = proxy.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: user,
		OTP:      &authclient.OTPCreds{Password: pass, Token: "wrong123"},
	})
	require.True(t, trace.IsAccessDenied(err))

	// authentication attempt fails with password auth only
	_, err = proxy.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: user,
		Pass: &authclient.PassCreds{
			Password: pass,
		},
	})
	require.True(t, trace.IsAccessDenied(err))

	// authentication succeeds
	ws, err := proxy.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: user,
		OTP:      &authclient.OTPCreds{Password: pass, Token: validToken},
	})
	require.NoError(t, err)

	userClient, err := testSrv.NewClientFromWebSession(ws)
	require.NoError(t, err)

	_, err = userClient.GetWebSessionInfo(ctx, user, ws.GetName())
	require.NoError(t, err)

	err = clt.DeleteWebSession(ctx, user, ws.GetName())
	require.NoError(t, err)

	_, err = userClient.GetWebSessionInfo(ctx, user, ws.GetName())
	require.Error(t, err)
}

// TestLoginAttempts makes sure the login attempt counter is incremented and
// reset correctly.
func TestLoginAttempts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	user := "user1"
	pass := []byte("abc123")

	_, _, err = CreateUserAndRole(clt, user, []string{user}, nil)
	require.NoError(t, err)

	proxy, err := testSrv.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	err = testSrv.Auth().UpsertPassword(user, pass)
	require.NoError(t, err)

	req := authclient.AuthenticateUserRequest{
		Username: user,
		Pass: &authclient.PassCreds{
			Password: []byte("bad pass"),
		},
	}
	// authentication attempt fails with bad password
	_, err = proxy.AuthenticateWebUser(ctx, req)
	require.True(t, trace.IsAccessDenied(err))

	// creates first failed login attempt
	loginAttempts, err := testSrv.Auth().GetUserLoginAttempts(user)
	require.NoError(t, err)
	require.Len(t, loginAttempts, 1)

	// try second time with wrong pass
	req.Pass.Password = pass
	_, err = proxy.AuthenticateWebUser(ctx, req)
	require.NoError(t, err)

	// clears all failed attempts after success
	loginAttempts, err = testSrv.Auth().GetUserLoginAttempts(user)
	require.NoError(t, err)
	require.Len(t, loginAttempts, 0)
}

func TestChangeUserAuthenticationSettings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		AllowLocalAuth: types.NewBoolOption(true),
	})
	require.NoError(t, err)

	err = testSrv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorWebauthn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)

	err = testSrv.Auth().SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	username := "user1"
	// Create a local user.
	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	_, err = CreateUser(clt, username)
	require.NoError(t, err)

	t.Run("Reset works when user exists", func(t *testing.T) {
		token, err := testSrv.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
			Name: username,
			TTL:  time.Hour,
		})
		require.NoError(t, err)

		res, err := testSrv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			TokenID:    token.GetName(),
			DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		})
		require.NoError(t, err)

		_, registerSolved, err := NewTestDeviceFromChallenge(res)
		require.NoError(t, err)

		_, err = testSrv.Auth().ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
			TokenID:                token.GetName(),
			NewPassword:            []byte("qweqweqwe"),
			NewMFARegisterResponse: registerSolved,
		})
		require.NoError(t, err)
	})

	t.Run("Reset link not allowed when user does not exist", func(t *testing.T) {
		token, err := testSrv.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
			Name: username,
			TTL:  time.Hour,
		})
		require.NoError(t, err)

		var resp *proto.MFARegisterResponse
		for i := 0; i < 5; i++ {
			res, err := testSrv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
				TokenID:    token.GetName(),
				DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			})
			require.NoError(t, err)

			_, registerSolved, err := NewTestDeviceFromChallenge(res)
			require.NoError(t, err)

			resp = registerSolved
		}

		require.NoError(t, testSrv.Auth().DeleteUser(ctx, username))

		_, err = testSrv.Auth().ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
			TokenID:                token.GetName(),
			NewPassword:            []byte("qweqweqwe"),
			NewMFARegisterResponse: resp,
		})
		require.Error(t, err)

		tokens, err := testSrv.Auth().GetUserTokens(ctx)
		require.NoError(t, err)
		require.Empty(t, tokens)
	})
}

// TestLoginNoLocalAuth makes sure that logins for local accounts can not be
// performed when local auth is disabled.
func TestLoginNoLocalAuth(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	user := "foo"
	pass := []byte("barbaz")

	// Create a local user.
	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)
	_, _, err = CreateUserAndRole(clt, user, []string{user}, nil)
	require.NoError(t, err)
	err = testSrv.Auth().UpsertPassword(user, pass)
	require.NoError(t, err)

	// Set auth preference to disallow local auth.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		AllowLocalAuth: types.NewBoolOption(false),
	})
	require.NoError(t, err)
	err = testSrv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Make sure access is denied for web login.
	_, err = testSrv.Auth().AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
		Username: user,
		Pass: &authclient.PassCreds{
			Password: pass,
		},
	})
	require.True(t, trace.IsAccessDenied(err))

	// Make sure access is denied for SSH login.
	_, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	_, err = testSrv.Auth().AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
		AuthenticateUserRequest: authclient.AuthenticateUserRequest{
			Username: user,
			Pass: &authclient.PassCreds{
				Password: pass,
			},
			PublicKey: pub,
		},
	})
	require.True(t, trace.IsAccessDenied(err))
}

// TestCipherSuites makes sure that clients with invalid cipher suites can
// not connect.
func TestCipherSuites(t *testing.T) {
	testSrv := newTestTLSServer(t)

	otherServer, err := testSrv.AuthServer.NewTestTLSServer()
	require.NoError(t, err)
	defer otherServer.Close()

	// Create a client with ciphersuites that the server does not support.
	tlsConfig, err := testSrv.ClientTLSConfig(TestNop())
	require.NoError(t, err)
	tlsConfig.CipherSuites = []uint16{
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	}

	addrs := []string{
		otherServer.Addr().String(),
		testSrv.Addr().String(),
	}
	client, err := authclient.NewClient(client.Config{
		Addrs: addrs,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: breaker.NoopBreakerConfig(),
	})
	require.NoError(t, err)

	// Requests should fail.
	_, err = client.GetClusterName()
	require.Error(t, err)
}

// TestTLSFailover tests HTTP client failover between two tls servers
func TestTLSFailover(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	otherServer, err := testSrv.AuthServer.NewTestTLSServer()
	require.NoError(t, err)
	defer otherServer.Close()

	tlsConfig, err := testSrv.ClientTLSConfig(TestNop())
	require.NoError(t, err)

	addrs := []string{
		otherServer.Addr().String(),
		testSrv.Addr().String(),
	}
	client, err := authclient.NewClient(client.Config{
		Addrs: addrs,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: breaker.NoopBreakerConfig(),
	})
	require.NoError(t, err)

	// couple of runs to get enough connections
	for i := 0; i < 4; i++ {
		_, err = client.Get(ctx, client.Endpoint("not", "exist"), url.Values{})
		require.True(t, trace.IsNotFound(err))
	}

	// stop the server to get response
	err = otherServer.Stop()
	require.NoError(t, err)

	// client detects closed sockets and reconnect to the backup server
	for i := 0; i < 4; i++ {
		_, err = client.Get(ctx, client.Endpoint("not", "exist"), url.Values{})
		require.True(t, trace.IsNotFound(err))
	}
}

// TestRegisterCAPin makes sure that registration only works with a valid
// CA pin.
func TestRegisterCAPin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock

	// Generate a token to use.
	token := generateTestToken(
		ctx,
		t,
		types.SystemRoles{types.RoleProxy},
		time.Time{},
		testSrv.Auth(),
	)

	// Generate public and private keys for node.
	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	require.NoError(t, err)
	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	require.NoError(t, err)

	// Calculate what CA pin should be.
	localCAResponse, err := testSrv.AuthServer.AuthServer.GetClusterCACert(ctx)
	require.NoError(t, err)
	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	require.NoError(t, err)
	require.Len(t, caPins, 1)
	caPin := caPins[0]

	// Attempt to register with valid CA pin, should work.
	_, err = join.Register(ctx, join.RegisterParams{
		AuthServers: []utils.NetAddr{utils.FromAddr(testSrv.Addr())},
		Token:       token,
		ID: state.IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               []string{caPin},
		Clock:                clock,
	})
	require.NoError(t, err)

	// Attempt to register with multiple CA pins where the auth server only
	// matches one, should work.
	_, err = join.Register(ctx, join.RegisterParams{
		AuthServers: []utils.NetAddr{utils.FromAddr(testSrv.Addr())},
		Token:       token,
		ID: state.IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               []string{"sha256:123", caPin},
		Clock:                clock,
	})
	require.NoError(t, err)

	// Attempt to register with invalid CA pin, should fail.
	_, err = join.Register(ctx, join.RegisterParams{
		AuthServers: []utils.NetAddr{utils.FromAddr(testSrv.Addr())},
		Token:       token,
		ID: state.IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               []string{"sha256:123"},
		Clock:                clock,
	})
	require.Error(t, err)

	// Attempt to register with multiple invalid CA pins, should fail.
	_, err = join.Register(ctx, join.RegisterParams{
		AuthServers: []utils.NetAddr{utils.FromAddr(testSrv.Addr())},
		Token:       token,
		ID: state.IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               []string{"sha256:123", "sha256:456"},
		Clock:                clock,
	})
	require.Error(t, err)

	// Add another cert to the CA (dupe the current one for simplicity)
	hostCA, err := testSrv.AuthServer.AuthServer.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.AuthServer.ClusterName,
		Type:       types.HostCA,
	}, true)
	require.NoError(t, err)
	activeKeys := hostCA.GetActiveKeys()
	activeKeys.TLS = append(activeKeys.TLS, activeKeys.TLS...)
	hostCA.SetActiveKeys(activeKeys)
	err = testSrv.AuthServer.AuthServer.UpsertCertAuthority(ctx, hostCA)
	require.NoError(t, err)

	// Calculate what CA pins should be.
	localCAResponse, err = testSrv.AuthServer.AuthServer.GetClusterCACert(ctx)
	require.NoError(t, err)
	caPins, err = tlsca.CalculatePins(localCAResponse.TLSCA)
	require.NoError(t, err)
	require.Len(t, caPins, 2)

	// Attempt to register with multiple CA pins, should work
	_, err = join.Register(ctx, join.RegisterParams{
		AuthServers: []utils.NetAddr{utils.FromAddr(testSrv.Addr())},
		Token:       token,
		ID: state.IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               caPins,
		Clock:                clock,
	})
	require.NoError(t, err)
}

// TestRegisterCAPath makes sure registration only works with a valid CA
// file on disk.
func TestRegisterCAPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	clock := testSrv.AuthServer.TestAuthServerConfig.Clock
	dataDir := testSrv.AuthServer.Dir

	// Generate a token to use.
	token := generateTestToken(
		ctx,
		t,
		types.SystemRoles{types.RoleProxy},
		time.Time{},
		testSrv.Auth(),
	)

	// Generate public and private keys for node.
	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	require.NoError(t, err)
	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	require.NoError(t, err)

	// Attempt to register with nothing at the CA path, should work.
	_, err = join.Register(ctx, join.RegisterParams{
		AuthServers: []utils.NetAddr{utils.FromAddr(testSrv.Addr())},
		Token:       token,
		ID: state.IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		Clock:                clock,
	})
	require.NoError(t, err)

	// Extract the root CA public key and write it out to the data dir.
	hostCA, err := testSrv.AuthServer.AuthServer.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.AuthServer.ClusterName,
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)
	certs := services.GetTLSCerts(hostCA)
	require.Len(t, certs, 1)
	certPem := certs[0]
	caPath := filepath.Join(dataDir, defaults.CACertFile)
	err = os.WriteFile(caPath, certPem, teleport.FileMaskOwnerOnly)
	require.NoError(t, err)

	// Attempt to register with valid CA path, should work.
	_, err = join.Register(ctx, join.RegisterParams{
		AuthServers: []utils.NetAddr{utils.FromAddr(testSrv.Addr())},
		Token:       token,
		ID: state.IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPath:               caPath,
		Clock:                clock,
	})
	require.NoError(t, err)
}

func TestClusterAlertAck(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testSrv := newTestTLSServer(t)

	alert1, err := types.NewClusterAlert("alert-1", "some msg")
	require.NoError(t, err)

	adminClt, err := testSrv.NewClient(TestBuiltin(types.RoleAdmin))
	require.NoError(t, err)
	defer adminClt.Close()

	err = adminClt.UpsertClusterAlert(ctx, alert1)
	require.NoError(t, err)

	expiry := time.Now().Add(time.Hour)

	ack := types.AlertAcknowledgement{AlertID: "alert-1", Reason: "testing", Expires: expiry}

	err = adminClt.CreateAlertAck(ctx, ack)
	require.NoError(t, err)

	acks, err := adminClt.GetAlertAcks(ctx)
	require.NoError(t, err)

	require.Len(t, acks, 1)

	require.Equal(t, acks[0].AlertID, "alert-1")

	clear := proto.ClearAlertAcksRequest{
		AlertID: "alert-1",
	}

	err = adminClt.ClearAlertAcks(ctx, clear)
	require.NoError(t, err)

	acks, err = adminClt.GetAlertAcks(ctx)
	require.NoError(t, err)

	require.Len(t, acks, 0)
}

func TestClusterAlertClearAckWildcard(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testSrv := newTestTLSServer(t)

	alert1, err := types.NewClusterAlert("alert-1", "some msg")
	require.NoError(t, err)

	alert2, err := types.NewClusterAlert("alert-2", "some msg")
	require.NoError(t, err)

	adminClt, err := testSrv.NewClient(TestBuiltin(types.RoleAdmin))
	require.NoError(t, err)
	defer adminClt.Close()

	err = adminClt.UpsertClusterAlert(ctx, alert1)
	require.NoError(t, err)

	err = adminClt.UpsertClusterAlert(ctx, alert2)
	require.NoError(t, err)

	expiry := time.Now().Add(time.Hour)

	ack := types.AlertAcknowledgement{AlertID: "alert-1", Reason: "testing", Expires: expiry}

	err = adminClt.CreateAlertAck(ctx, ack)
	require.NoError(t, err)

	ack = types.AlertAcknowledgement{AlertID: "alert-2", Reason: "testing", Expires: expiry}

	err = adminClt.CreateAlertAck(ctx, ack)
	require.NoError(t, err)

	acks, err := adminClt.GetAlertAcks(ctx)
	require.NoError(t, err)

	require.Len(t, acks, 2)

	require.Equal(t, acks[0].AlertID, "alert-1")
	require.Equal(t, acks[1].AlertID, "alert-2")

	clear := proto.ClearAlertAcksRequest{
		AlertID: "*",
	}

	err = adminClt.ClearAlertAcks(ctx, clear)
	require.NoError(t, err)

	acks, err = adminClt.GetAlertAcks(ctx)
	require.NoError(t, err)

	require.Len(t, acks, 0)
}

// TestClusterAlertAccessControls verifies expected behaviors of cluster alert
// access controls.
func TestClusterAlertAccessControls(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testSrv := newTestTLSServer(t)

	expectAlerts := func(alerts []types.ClusterAlert, names ...string) {
		for _, alert := range alerts {
			for _, name := range names {
				if alert.Metadata.Name == name {
					return
				}
			}
			t.Fatalf("unexpected alert %q", alert.Metadata.Name)
		}
	}

	alert1, err := types.NewClusterAlert("alert-1", "some msg")
	require.NoError(t, err)

	alert2, err := types.NewClusterAlert("alert-2", "other msg")
	require.NoError(t, err)

	// set one of the two alerts to be viewable by all users
	alert2.Metadata.Labels = map[string]string{
		types.AlertPermitAll: "yes",
	}

	adminClt, err := testSrv.NewClient(TestBuiltin(types.RoleAdmin))
	require.NoError(t, err)
	defer adminClt.Close()

	err = adminClt.UpsertClusterAlert(ctx, alert1)
	require.NoError(t, err)

	err = adminClt.UpsertClusterAlert(ctx, alert2)
	require.NoError(t, err)

	// verify that admin client can see all alerts due to resource-level permissions
	alerts, err := adminClt.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		WithUntargeted: true,
	})
	require.NoError(t, err)
	require.Len(t, alerts, 2)
	expectAlerts(alerts, "alert-1", "alert-2")

	// verify that WithUntargeted=false admin only observes the alert that specifies
	// that it should be shown to all
	alerts, err = adminClt.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		WithUntargeted: false,
	})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	expectAlerts(alerts, "alert-2")

	// verify that some other client with no alert-specific permissions can
	// see the "permit-all" subset of alerts (using role node here, but any
	// role with no special provisions for alerts should be equivalent)
	otherClt, err := testSrv.NewClient(TestBuiltin(types.RoleNode))
	require.NoError(t, err)
	defer otherClt.Close()

	// untargeted and targeted should result in the same behavior since otherClt
	// does not have resource-level permissions for the cluster_alert type.
	for _, untargeted := range []bool{true, false} {
		alerts, err = otherClt.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			WithUntargeted: untargeted,
		})
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		expectAlerts(alerts, "alert-2")
	}

	// verify that we still reject unauthenticated clients
	nopClt, err := testSrv.NewClient(TestBuiltin(types.RoleNop))
	require.NoError(t, err)
	defer nopClt.Close()

	_, err = nopClt.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.True(t, trace.IsAccessDenied(err))

	// add more alerts that use resource verb permits
	alert3, err := types.NewClusterAlert(
		"alert-3",
		"msg 3",
		types.WithAlertLabel(types.AlertVerbPermit, "token:create"),
	)
	require.NoError(t, err)

	alert4, err := types.NewClusterAlert(
		"alert-4",
		"msg 4",
		types.WithAlertLabel(types.AlertVerbPermit, "token:create|role:read"),
	)
	require.NoError(t, err)

	err = adminClt.UpsertClusterAlert(ctx, alert3)
	require.NoError(t, err)

	err = adminClt.UpsertClusterAlert(ctx, alert4)
	require.NoError(t, err)

	// verify that admin client can see all alerts in untargeted read mode
	alerts, err = adminClt.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		WithUntargeted: true,
	})
	require.NoError(t, err)
	require.Len(t, alerts, 4)
	expectAlerts(alerts, "alert-1", "alert-2", "alert-3", "alert-4")

	// verify that admin client can see all targeted alerts in targeted mode
	alerts, err = adminClt.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		WithUntargeted: false,
	})
	require.NoError(t, err)
	require.Len(t, alerts, 3)
	expectAlerts(alerts, "alert-2", "alert-3", "alert-4")

	// verify that node client can only see one of the two new alerts
	alerts, err = otherClt.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	require.Len(t, alerts, 2)
	expectAlerts(alerts, "alert-2", "alert-4")
}

// TestEventsNodePresence tests streaming node presence API -
// announcing node and keeping node alive
func TestEventsNodePresence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "node1",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.ServerSpecV2{
			Addr: "localhost:3022",
		},
	}
	node.SetExpiry(time.Now().Add(2 * time.Second))
	clt, err := testSrv.NewClient(TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleNode,
			Username: fmt.Sprintf("%v.%v", node.Metadata.Name, testSrv.ClusterName()),
		},
	})
	require.NoError(t, err)
	defer clt.Close()

	keepAlive, err := clt.UpsertNode(ctx, node)
	require.NoError(t, err)
	require.NotNil(t, keepAlive)

	keepAliver, err := clt.NewKeepAliver(ctx)
	require.NoError(t, err)
	defer keepAliver.Close()

	keepAlive.Expires = time.Now().Add(2 * time.Second)
	select {
	case keepAliver.KeepAlives() <- *keepAlive:
		// ok
	case <-time.After(time.Second):
		t.Fatalf("time out sending keep alive")
	case <-keepAliver.Done():
		t.Fatalf("unknown problem sending keep alive")
	}

	// upsert node and keep alives will fail for users with no privileges
	nopClt, err := testSrv.NewClient(TestBuiltin(types.RoleNop))
	require.NoError(t, err)
	defer nopClt.Close()

	_, err = nopClt.UpsertNode(ctx, node)
	require.True(t, trace.IsAccessDenied(err))

	k2, err := nopClt.NewKeepAliver(ctx)
	require.NoError(t, err)

	keepAlive.Expires = time.Now().Add(2 * time.Second)
	go func() {
		select {
		case k2.KeepAlives() <- *keepAlive:
		case <-k2.Done():
		}
	}()

	select {
	case <-time.After(time.Second):
		t.Fatalf("time out expecting error")
	case <-k2.Done():
	}

	require.True(t, trace.IsAccessDenied(k2.Error()))
}

// TestEventsPermissions tests events with regards
// to certificate authority rotation
func TestEventsPermissions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestBuiltin(types.RoleNode))
	require.NoError(t, err)
	defer clt.Close()

	w, err := clt.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{{Kind: types.KindCertAuthority}}})
	require.NoError(t, err)
	defer w.Close()

	select {
	case <-time.After(2 * time.Second):
		t.Fatalf("Timeout waiting for init event")
	case event := <-w.Events():
		require.Equal(t, event.Type, types.OpInit)
	}

	// start rotation
	gracePeriod := time.Hour
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	ca, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, false)
	require.NoError(t, err)

	suite.ExpectResource(t, w, 3*time.Second, ca)

	type testCase struct {
		name     string
		identity TestIdentity
		watches  []types.WatchKind
	}

	testCases := []testCase{
		{
			name:     "node role is not authorized to get certificate authority with secret data loaded",
			identity: TestBuiltin(types.RoleNode),
			watches:  []types.WatchKind{{Kind: types.KindCertAuthority, LoadSecrets: true}},
		},
		{
			name:     "node role is not authorized to watch static tokens",
			identity: TestBuiltin(types.RoleNode),
			watches:  []types.WatchKind{{Kind: types.KindStaticTokens}},
		},
		{
			name:     "node role is not authorized to watch provisioning tokens",
			identity: TestBuiltin(types.RoleNode),
			watches:  []types.WatchKind{{Kind: types.KindToken}},
		},
		{
			name:     "nop role is not authorized to watch users and roles",
			identity: TestBuiltin(types.RoleNop),
			watches: []types.WatchKind{
				{Kind: types.KindUser},
				{Kind: types.KindRole},
			},
		},
		{
			name:     "nop role is not authorized to watch cert authorities",
			identity: TestBuiltin(types.RoleNop),
			watches:  []types.WatchKind{{Kind: types.KindCertAuthority, LoadSecrets: false}},
		},
		{
			name:     "nop role is not authorized to watch cluster config resources",
			identity: TestBuiltin(types.RoleNop),
			watches: []types.WatchKind{
				{Kind: types.KindClusterAuthPreference},
				{Kind: types.KindClusterNetworkingConfig},
				{Kind: types.KindSessionRecordingConfig},
			},
		},
	}

	tryWatch := func(tc testCase) {
		client, err := testSrv.NewClient(tc.identity)
		require.NoError(t, err)
		defer client.Close()

		watcher, err := client.NewWatcher(ctx, types.Watch{
			Kinds: tc.watches,
		})
		require.NoError(t, err)
		defer watcher.Close()

		go func() {
			select {
			case <-watcher.Events():
			case <-watcher.Done():
			}
		}()

		select {
		case <-time.After(time.Second):
			t.Fatalf("time out expecting error in test %q", tc.name)
		case <-watcher.Done():
		}

		require.True(t, trace.IsAccessDenied(watcher.Error()))
	}

	for _, tc := range testCases {
		tryWatch(tc)
	}
}

// TestEventsPermissionsPartialSuccess verifies that in partial success mode NewWatcher can still succeed
// if caller lacks permission to watch only some of the requested resource kinds.
func TestEventsPermissionsPartialSuccess(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                   string
		watch                  types.Watch
		expectedConfirmedKinds []types.WatchKind
	}{
		{
			name: "no permission for any of the requested kinds",
			watch: types.Watch{
				Kinds: []types.WatchKind{
					{Kind: types.KindUser},
					{Kind: types.KindRole},
				},
				AllowPartialSuccess: true,
			},
		},
		{
			name: "has permission only for some of the requested kinds",
			watch: types.Watch{
				Kinds: []types.WatchKind{
					{Kind: types.KindUser},
					{Kind: types.KindRole},
					{Kind: types.KindStaticTokens},
				},
				AllowPartialSuccess: true,
			},
			expectedConfirmedKinds: []types.WatchKind{
				{Kind: types.KindStaticTokens},
			},
		},
		{
			name: "has permission only for some kinds but partial success is not enabled",
			watch: types.Watch{
				Kinds: []types.WatchKind{
					{Kind: types.KindUser},
					{Kind: types.KindRole},
					{Kind: types.KindStaticTokens},
				},
			},
		},
	}

	ctx := context.Background()
	testSrv := newTestTLSServer(t)
	testUser, testRole, err := CreateUserAndRole(testSrv.Auth(), "test", nil, []types.Rule{
		types.NewRule(types.KindStaticTokens, services.RO()),
	})
	require.NoError(t, err)
	require.NoError(t, testSrv.Auth().UpsertRole(ctx, testRole))
	testIdentity := TestUser(testUser.GetName())

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := testSrv.NewClient(testIdentity)
			require.NoError(t, err)
			defer client.Close()

			w, err := client.NewWatcher(ctx, tc.watch)
			require.NoError(t, err)
			defer w.Close()

			select {
			case event := <-w.Events():
				if len(tc.expectedConfirmedKinds) > 0 {
					require.Equal(t, event.Type, types.OpInit)
					watchStatus, ok := event.Resource.(types.WatchStatus)
					require.True(t, ok)
					require.Equal(t, tc.expectedConfirmedKinds, watchStatus.GetKinds())
				} else {
					t.Fatal("unexpected event from watcher that is supposed to fail")
				}
			case <-w.Done():
				if len(tc.expectedConfirmedKinds) > 0 {
					t.Fatalf("Watcher exited with error %v", w.Error())
				}
				require.Error(t, w.Error())
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for watcher")
			}
		})
	}
}

// TestEvents tests events suite
func TestEvents(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	suite := &suite.ServicesTestSuite{
		ConfigS:       clt,
		EventsS:       clt,
		PresenceS:     clt,
		CAS:           clt,
		ProvisioningS: clt,
		Access:        clt,
		UsersS:        clt,
	}
	suite.Events(t)
}

// TestEventsClusterConfig test cluster configuration
func TestEventsClusterConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestBuiltin(types.RoleAdmin))
	require.NoError(t, err)
	defer clt.Close()

	w, err := clt.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: true},
		{Kind: types.KindStaticTokens},
		{Kind: types.KindToken},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterName},
	}})
	require.NoError(t, err)
	defer w.Close()

	select {
	case <-time.After(2 * time.Second):
		t.Fatalf("Timeout waiting for init event")
	case event := <-w.Events():
		require.Equal(t, event.Type, types.OpInit)
	}

	// start rotation
	gracePeriod := time.Hour
	err = testSrv.Auth().RotateCertAuthority(ctx, RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	ca, err := testSrv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: testSrv.ClusterName(),
		Type:       types.HostCA,
	}, true)
	require.NoError(t, err)

	suite.ExpectResource(t, w, 3*time.Second, ca)

	// set static tokens
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Token:   "tok1",
				Roles:   types.SystemRoles{types.RoleNode},
				Expires: time.Now().UTC().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	err = testSrv.Auth().SetStaticTokens(staticTokens)
	require.NoError(t, err)

	staticTokens, err = testSrv.Auth().GetStaticTokens()
	require.NoError(t, err)
	suite.ExpectResource(t, w, 3*time.Second, staticTokens)

	// create provision token and expect the update event
	token, err := types.NewProvisionToken(
		"tok2", types.SystemRoles{types.RoleProxy}, time.Now().UTC().Add(3*time.Hour))
	require.NoError(t, err)

	err = testSrv.Auth().UpsertToken(ctx, token)
	require.NoError(t, err)

	token, err = testSrv.Auth().GetToken(ctx, token.GetName())
	require.NoError(t, err)

	suite.ExpectResource(t, w, 3*time.Second, token)

	// delete token and expect delete event
	err = testSrv.Auth().DeleteToken(ctx, token.GetName())
	require.NoError(t, err)
	suite.ExpectDeleteResource(t, w, 3*time.Second, &types.ResourceHeader{
		Kind:    types.KindToken,
		Version: types.V2,
		Metadata: types.Metadata{
			Namespace: apidefaults.Namespace,
			Name:      token.GetName(),
		},
	})

	// update audit config
	auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
		AuditEventsURI: []string{"dynamodb://audit_table_name", "file:///home/log"},
	})
	require.NoError(t, err)
	err = testSrv.Auth().SetClusterAuditConfig(ctx, auditConfig)
	require.NoError(t, err)

	auditConfigResource, err := testSrv.Auth().GetClusterAuditConfig(ctx)
	require.NoError(t, err)
	suite.ExpectResource(t, w, 3*time.Second, auditConfigResource)

	// update cluster name resource metadata
	clusterNameResource, err := testSrv.Auth().GetClusterName()
	require.NoError(t, err)

	// update the resource with different labels to test the change
	clusterName := &types.ClusterNameV2{
		Kind:    types.KindClusterName,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      types.MetaNameClusterName,
			Namespace: apidefaults.Namespace,
			Labels: map[string]string{
				"key": "val",
			},
		},
		Spec: clusterNameResource.(*types.ClusterNameV2).Spec,
	}

	err = testSrv.Auth().DeleteClusterName()
	require.NoError(t, err)
	err = testSrv.Auth().SetClusterName(clusterName)
	require.NoError(t, err)

	clusterNameResource, err = testSrv.Auth().GetClusterName()
	require.NoError(t, err)
	suite.ExpectResource(t, w, 3*time.Second, clusterNameResource)
}

func TestNetworkRestrictions(t *testing.T) {
	t.Parallel()

	testSrv := newTestTLSServer(t)

	clt, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)

	suite := &suite.ServicesTestSuite{
		RestrictionsS: clt,
	}
	suite.NetworkRestrictions(t)
}

func mustNewToken(
	t *testing.T,
	token string,
	roles types.SystemRoles,
	expires time.Time,
) types.ProvisionToken {
	tok, err := types.NewProvisionToken(token, roles, expires)
	require.NoError(t, err)
	return tok
}

func mustNewTokenFromSpec(
	t *testing.T,
	token string,
	expires time.Time,
	spec types.ProvisionTokenSpecV2,
) types.ProvisionToken {
	tok, err := types.NewProvisionTokenFromSpec(token, expires, spec)
	require.NoError(t, err)
	return tok
}

func requireAccessDenied(t require.TestingT, err error, i ...interface{}) {
	require.True(
		t,
		trace.IsAccessDenied(err),
		"err should be access denied, was: %s", err,
	)
}

func requireBadParameter(t require.TestingT, err error, i ...interface{}) {
	require.True(
		t,
		trace.IsBadParameter(err),
		"err should be bad parameter, was: %s", err,
	)
}

func requireNotFound(t require.TestingT, err error, i ...interface{}) {
	require.True(
		t,
		trace.IsNotFound(err),
		"err should be not found, was: %s", err,
	)
}

func TestGRPCServer_CreateTokenV2(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	// Inject mockEmitter to capture audit event for trusted cluster
	// creation.
	mockEmitter := &eventstest.MockRecorderEmitter{}
	testSrv.Auth().SetEmitter(mockEmitter)

	// Create a user with the least privilege access to call this RPC.
	privilegedUser, _, err := CreateUserAndRole(
		testSrv.Auth(), "token-creator", nil, []types.Rule{
			{
				Resources: []string{types.KindToken},
				Verbs:     []string{types.VerbCreate},
			},
		},
	)
	require.NoError(t, err)

	// create a token to conflict with for already having been created
	alreadyExistsToken := mustNewToken(
		t, "already-exists", types.SystemRoles{types.RoleNode}, time.Time{},
	)
	require.NoError(t, testSrv.Auth().CreateToken(ctx, alreadyExistsToken))

	tests := []struct {
		name     string
		identity TestIdentity
		token    types.ProvisionToken

		requireTokenCreated bool
		requireError        require.ErrorAssertionFunc
		auditEvents         []eventtypes.AuditEvent
	}{
		{
			name:     "success",
			identity: TestUser(privilegedUser.GetName()),
			token: mustNewTokenFromSpec(
				t,
				"success",
				time.Time{},
				types.ProvisionTokenSpecV2{
					Roles:      types.SystemRoles{types.RoleNode, types.RoleKube},
					JoinMethod: types.JoinMethodToken,
				},
			),
			requireError:        require.NoError,
			requireTokenCreated: true,
			auditEvents: []eventtypes.AuditEvent{
				&eventtypes.ProvisionTokenCreate{
					Metadata: eventtypes.Metadata{
						Type: events.ProvisionTokenCreateEvent,
						Code: events.ProvisionTokenCreateCode,
					},
					UserMetadata: eventtypes.UserMetadata{
						User:     "token-creator",
						UserKind: eventtypes.UserKind_USER_KIND_HUMAN,
					},
					Roles:      types.SystemRoles{types.RoleNode, types.RoleKube},
					JoinMethod: types.JoinMethodToken,
				},
			},
		},
		{
			name:     "success (trusted cluster)",
			identity: TestUser(privilegedUser.GetName()),
			token: mustNewTokenFromSpec(
				t,
				"success-trusted-cluster",
				time.Time{},
				types.ProvisionTokenSpecV2{
					Roles:      types.SystemRoles{types.RoleTrustedCluster},
					JoinMethod: types.JoinMethodToken,
				},
			),
			requireError:        require.NoError,
			requireTokenCreated: true,
			auditEvents: []eventtypes.AuditEvent{
				&eventtypes.ProvisionTokenCreate{
					Metadata: eventtypes.Metadata{
						Type: events.ProvisionTokenCreateEvent,
						Code: events.ProvisionTokenCreateCode,
					},
					UserMetadata: eventtypes.UserMetadata{
						User:     "token-creator",
						UserKind: eventtypes.UserKind_USER_KIND_HUMAN,
					},
					Roles:      types.SystemRoles{types.RoleTrustedCluster},
					JoinMethod: types.JoinMethodToken,
				},
				//nolint:staticcheck // Emit a deprecated event.
				&eventtypes.TrustedClusterTokenCreate{
					Metadata: eventtypes.Metadata{
						Type: events.TrustedClusterTokenCreateEvent,
						Code: events.TrustedClusterTokenCreateCode,
					},
					UserMetadata: eventtypes.UserMetadata{
						User:     "token-creator",
						UserKind: eventtypes.UserKind_USER_KIND_HUMAN,
					},
				},
			},
		},
		{
			name:     "access denied",
			identity: TestNop(),
			token: mustNewToken(
				t, "access denied", types.SystemRoles{types.RoleNode}, time.Time{},
			),
			requireError: requireAccessDenied,
		},
		{
			name:     "already exists",
			identity: TestUser(privilegedUser.GetName()),
			token:    alreadyExistsToken,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(
					t,
					trace.IsAlreadyExists(err),
					"err should be already exists, was: %s", err,
				)
			},
		},
		{
			name:         "invalid token",
			identity:     TestUser(privilegedUser.GetName()),
			token:        &types.ProvisionTokenV2{},
			requireError: requireBadParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := testSrv.NewClient(tt.identity)
			require.NoError(t, err)

			mockEmitter.Reset()
			err = client.CreateToken(ctx, tt.token)
			tt.requireError(t, err)

			require.Empty(t, cmp.Diff(
				tt.auditEvents,
				mockEmitter.Events(),
				cmpopts.IgnoreFields(eventtypes.Metadata{}, "Time"),
				cmpopts.IgnoreFields(eventtypes.ResourceMetadata{}, "Expires"),
				cmpopts.EquateEmpty(),
			))
			if tt.requireTokenCreated {
				token, err := testSrv.Auth().GetToken(ctx, tt.token.GetName())
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(
					tt.token,
					token,
					cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
				))
			}
		})
	}
}

func TestGRPCServer_UpsertTokenV2(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	// Inject mockEmitter to capture audit event for trusted cluster
	// creation.
	mockEmitter := &eventstest.MockRecorderEmitter{}
	testSrv.Auth().SetEmitter(mockEmitter)

	// Create a user with the least privilege access to call this RPC.
	privilegedUser, _, err := CreateUserAndRole(
		testSrv.Auth(), "token-upserter", nil, []types.Rule{
			{
				Resources: []string{types.KindToken},
				Verbs:     []string{types.VerbCreate, types.VerbUpdate},
			},
		},
	)
	require.NoError(t, err)

	// create a token to ensure UpsertToken overwrites an existing token.
	alreadyExistsToken := mustNewToken(
		t, "already-exists", types.SystemRoles{types.RoleNode}, time.Time{},
	)
	require.NoError(t, testSrv.Auth().CreateToken(ctx, alreadyExistsToken))

	tests := []struct {
		name     string
		identity TestIdentity
		token    types.ProvisionToken

		requireTokenCreated bool
		requireError        require.ErrorAssertionFunc
		auditEvents         []eventtypes.AuditEvent
	}{
		{
			name:     "success",
			identity: TestUser(privilegedUser.GetName()),
			token: mustNewTokenFromSpec(
				t,
				"success",
				time.Time{},
				types.ProvisionTokenSpecV2{
					Roles:      types.SystemRoles{types.RoleNode, types.RoleKube},
					JoinMethod: types.JoinMethodToken,
				},
			),
			requireError:        require.NoError,
			requireTokenCreated: true,
			auditEvents: []eventtypes.AuditEvent{
				&eventtypes.ProvisionTokenCreate{
					Metadata: eventtypes.Metadata{
						Type: events.ProvisionTokenCreateEvent,
						Code: events.ProvisionTokenCreateCode,
					},
					UserMetadata: eventtypes.UserMetadata{
						User:     "token-upserter",
						UserKind: eventtypes.UserKind_USER_KIND_HUMAN,
					},
					Roles:      types.SystemRoles{types.RoleNode, types.RoleKube},
					JoinMethod: types.JoinMethodToken,
				},
			},
		},
		{
			name:     "success (trusted cluster)",
			identity: TestUser(privilegedUser.GetName()),
			token: mustNewTokenFromSpec(
				t,
				"success-trusted-cluster",
				time.Time{},
				types.ProvisionTokenSpecV2{
					Roles:      types.SystemRoles{types.RoleTrustedCluster},
					JoinMethod: types.JoinMethodToken,
				},
			),
			requireError:        require.NoError,
			requireTokenCreated: true,
			auditEvents: []eventtypes.AuditEvent{
				&eventtypes.ProvisionTokenCreate{
					Metadata: eventtypes.Metadata{
						Type: events.ProvisionTokenCreateEvent,
						Code: events.ProvisionTokenCreateCode,
					},
					UserMetadata: eventtypes.UserMetadata{
						User:     "token-upserter",
						UserKind: eventtypes.UserKind_USER_KIND_HUMAN,
					},
					Roles:      types.SystemRoles{types.RoleTrustedCluster},
					JoinMethod: types.JoinMethodToken,
				},
				//nolint:staticcheck // Emit a deprecated event.
				&eventtypes.TrustedClusterTokenCreate{
					Metadata: eventtypes.Metadata{
						Type: events.TrustedClusterTokenCreateEvent,
						Code: events.TrustedClusterTokenCreateCode,
					},
					UserMetadata: eventtypes.UserMetadata{
						User:     "token-upserter",
						UserKind: eventtypes.UserKind_USER_KIND_HUMAN,
					},
				},
			},
		},
		{
			name:     "existing token replaced",
			identity: TestUser(privilegedUser.GetName()),
			token: mustNewTokenFromSpec(
				t,
				alreadyExistsToken.GetName(),
				time.Time{},
				types.ProvisionTokenSpecV2{
					// These roles differ from the roles on the already existing
					// token.
					Roles:      types.SystemRoles{types.RoleNode},
					JoinMethod: types.JoinMethodToken,
				},
			),
			requireTokenCreated: true,
			requireError:        require.NoError,
			auditEvents: []eventtypes.AuditEvent{
				&eventtypes.ProvisionTokenCreate{
					Metadata: eventtypes.Metadata{
						Type: events.ProvisionTokenCreateEvent,
						Code: events.ProvisionTokenCreateCode,
					},
					UserMetadata: eventtypes.UserMetadata{
						User:     "token-upserter",
						UserKind: eventtypes.UserKind_USER_KIND_HUMAN,
					},
					Roles:      types.SystemRoles{types.RoleNode},
					JoinMethod: types.JoinMethodToken,
				},
			},
		},
		{
			name:     "access denied",
			identity: TestNop(),
			token: mustNewToken(
				t, "access denied", types.SystemRoles{types.RoleNode}, time.Time{},
			),
			requireError: requireAccessDenied,
		},
		{
			name:         "invalid token",
			identity:     TestUser(privilegedUser.GetName()),
			token:        &types.ProvisionTokenV2{},
			requireError: requireBadParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := testSrv.NewClient(tt.identity)
			require.NoError(t, err)

			mockEmitter.Reset()
			err = client.UpsertToken(ctx, tt.token)
			tt.requireError(t, err)

			require.Empty(t, cmp.Diff(
				tt.auditEvents,
				mockEmitter.Events(),
				cmpopts.IgnoreFields(eventtypes.Metadata{}, "Time"),
				cmpopts.IgnoreFields(eventtypes.ResourceMetadata{}, "Expires"),
				cmpopts.EquateEmpty(),
			))
			if tt.requireTokenCreated {
				token, err := testSrv.Auth().GetToken(ctx, tt.token.GetName())
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(
					tt.token,
					token,
					cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
				))
			}
		})
	}
}

func TestGRPCServer_GetTokens(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	// Create a user with the least privilege access to call this RPC.
	privilegedUser, _, err := CreateUserAndRole(
		testSrv.Auth(), "token-reader", nil, []types.Rule{
			{
				Resources: []string{types.KindToken},
				Verbs:     []string{types.VerbRead, types.VerbList},
			},
		},
	)
	require.NoError(t, err)

	t.Run("no tokens", func(t *testing.T) {
		client, err := testSrv.NewClient(TestUser(privilegedUser.GetName()))
		require.NoError(t, err)
		toks, err := client.GetTokens(ctx)
		require.NoError(t, err)
		require.Empty(t, toks)
	})

	// Create tokens to then assert are returned
	pt := mustNewToken(
		t,
		"example-token",
		types.SystemRoles{types.RoleNode},
		time.Time{},
	)
	require.NoError(t, testSrv.Auth().CreateToken(ctx, pt))
	pt2 := mustNewToken(
		t,
		"example-token-2",
		types.SystemRoles{types.RoleNode},
		time.Time{},
	)
	require.NoError(t, testSrv.Auth().CreateToken(ctx, pt2))
	st, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles: types.SystemRoles{types.RoleProxy},
				Token: "example-static-token",
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, testSrv.Auth().SetStaticTokens(st))
	expectTokens := append([]types.ProvisionToken{pt, pt2}, st.GetStaticTokens()...)

	tests := []struct {
		name     string
		identity TestIdentity

		requireResponse bool
		requireError    require.ErrorAssertionFunc
	}{
		{
			name:            "success",
			identity:        TestUser(privilegedUser.GetName()),
			requireError:    require.NoError,
			requireResponse: true,
		},
		{
			name:         "access denied",
			identity:     TestNop(),
			requireError: requireAccessDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := testSrv.NewClient(tt.identity)
			require.NoError(t, err)

			tokens, err := client.GetTokens(ctx)
			tt.requireError(t, err)

			if tt.requireResponse {
				require.Empty(t, cmp.Diff(
					expectTokens,
					tokens,
					cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
				))
			} else {
				require.Empty(t, tokens)
			}
		})
	}
}

func TestGRPCServer_GetToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	// Create a user with the least privilege access to call this RPC.
	privilegedUser, _, err := CreateUserAndRole(
		testSrv.Auth(), "token-reader", nil, []types.Rule{
			{
				Resources: []string{types.KindToken},
				Verbs:     []string{types.VerbRead},
			},
		},
	)
	require.NoError(t, err)

	// Create Provision token
	pt := mustNewToken(t, "example-token", types.SystemRoles{types.RoleNode}, time.Time{})
	require.NoError(t, testSrv.Auth().CreateToken(ctx, pt))

	tests := []struct {
		name      string
		tokenName string
		identity  TestIdentity

		requireResponse bool
		requireError    require.ErrorAssertionFunc
	}{
		{
			name:            "success",
			tokenName:       pt.GetName(),
			identity:        TestUser(privilegedUser.GetName()),
			requireError:    require.NoError,
			requireResponse: true,
		},
		{
			name:         "access denied",
			identity:     TestNop(),
			tokenName:    pt.GetName(),
			requireError: requireAccessDenied,
		},
		{
			name:         "not found",
			tokenName:    "does-not-exist",
			identity:     TestUser(privilegedUser.GetName()),
			requireError: requireNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := testSrv.NewClient(tt.identity)
			require.NoError(t, err)

			token, err := client.GetToken(ctx, tt.tokenName)
			tt.requireError(t, err)

			if tt.requireResponse {
				require.Empty(t, cmp.Diff(
					token,
					pt,
					cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
				))
			} else {
				require.Nil(t, token)
			}
		})
	}
}

func TestGRPCServer_DeleteToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	// Create a user with the least privilege access to call this RPC.
	privilegedUser, _, err := CreateUserAndRole(
		testSrv.Auth(), "token-deleter", nil, []types.Rule{
			{
				Resources: []string{types.KindToken},
				Verbs:     []string{types.VerbDelete},
			},
		},
	)
	require.NoError(t, err)

	// Create Provision token
	pt := mustNewToken(t, "example-token", types.SystemRoles{types.RoleNode}, time.Time{})
	require.NoError(t, testSrv.Auth().CreateToken(ctx, pt))

	tests := []struct {
		name      string
		tokenName string
		identity  TestIdentity

		requireTokenDeleted bool
		requireError        require.ErrorAssertionFunc
	}{
		{
			name:                "success",
			tokenName:           pt.GetName(),
			identity:            TestUser(privilegedUser.GetName()),
			requireError:        require.NoError,
			requireTokenDeleted: true,
		},
		{
			name:         "access denied",
			identity:     TestNop(),
			tokenName:    pt.GetName(),
			requireError: requireAccessDenied,
		},
		{
			name:         "not found",
			tokenName:    "does-not-exist",
			identity:     TestUser(privilegedUser.GetName()),
			requireError: requireNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := testSrv.NewClient(tt.identity)
			require.NoError(t, err)

			err = client.DeleteToken(ctx, tt.tokenName)
			tt.requireError(t, err)

			if tt.requireTokenDeleted {
				_, err := testSrv.Auth().GetToken(ctx, tt.tokenName)
				require.True(
					t,
					trace.IsNotFound(err),
					"expected token to be deleted",
				)
			}
		})
	}
}

// verifyJWT verifies that the token was signed by one the passed in key pair.
func verifyJWT(clock clockwork.Clock, clusterName string, pairs []*types.JWTKeyPair, token string) (*jwt.Claims, error) {
	errs := []error{}
	for _, pair := range pairs {
		publicKey, err := utils.ParsePublicKey(pair.PublicKey)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}

		key, err := jwt.New(&jwt.Config{
			Clock:       clock,
			PublicKey:   publicKey,
			Algorithm:   defaults.ApplicationTokenAlgorithm,
			ClusterName: clusterName,
		})
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		claims, err := key.Verify(jwt.VerifyParams{
			RawToken: token,
			Username: "foo",
			URI:      "https://localhost:8080",
		})
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		return claims, nil
	}
	return nil, trace.NewAggregate(errs...)
}

// verifyJWTAWSOIDC verifies that the token was signed by one the passed in key pair.
func verifyJWTAWSOIDC(clock clockwork.Clock, clusterName string, pairs []*types.JWTKeyPair, token, issuer string) (*jwt.Claims, error) {
	errs := []error{}
	for _, pair := range pairs {
		publicKey, err := utils.ParsePublicKey(pair.PublicKey)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}

		key, err := jwt.New(&jwt.Config{
			Clock:       clock,
			PublicKey:   publicKey,
			Algorithm:   defaults.ApplicationTokenAlgorithm,
			ClusterName: clusterName,
		})
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		claims, err := key.VerifyAWSOIDC(jwt.AWSOIDCVerifyParams{
			RawToken: token,
			Issuer:   issuer,
		})
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		return claims, nil
	}
	return nil, trace.NewAggregate(errs...)
}

type testTLSServerOptions struct {
	cacheEnabled bool
	accessGraph  *AccessGraphConfig
}

type testTLSServerOption func(*testTLSServerOptions)

func withCacheEnabled(enabled bool) testTLSServerOption {
	return func(options *testTLSServerOptions) {
		options.cacheEnabled = enabled
	}
}

func withAccessGraphConfig(cfg AccessGraphConfig) testTLSServerOption {
	return func(options *testTLSServerOptions) {
		options.accessGraph = &cfg
	}
}

// newTestTLSServer is a helper that returns a *TestTLSServer with sensible
// defaults for most tests that are exercising Auth Service RPCs.
//
// For more advanced use-cases, call NewTestAuthServer and NewTestTLSServer
// to provide a more detailed configuration.
func newTestTLSServer(t testing.TB, opts ...testTLSServerOption) *TestTLSServer {
	var options testTLSServerOptions
	for _, opt := range opts {
		opt(&options)
	}
	as, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:          t.TempDir(),
		Clock:        clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
		CacheEnabled: options.cacheEnabled,
	})
	require.NoError(t, err)
	var tlsServerOpts []TestTLSServerOption
	if options.accessGraph != nil {
		tlsServerOpts = append(tlsServerOpts, WithAccessGraphConfig(*options.accessGraph))
	}
	srv, err := as.NewTestTLSServer(tlsServerOpts...)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv
}

func TestVerifyPeerCert(t *testing.T) {
	t.Parallel()
	const (
		localClusterName  = "local"
		remoteClusterName = "remote"
	)
	s := newTestServices(t)
	// Set up local cluster name in the backend.
	cn, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: localClusterName,
	})
	require.NoError(t, err)
	require.NoError(t, s.UpsertClusterName(cn))

	now := time.Date(2020, time.November, 5, 0, 0, 0, 0, time.UTC)

	var (
		localUserIdentity = tlsca.Identity{
			Username:        "foo",
			Groups:          []string{"devs"},
			TeleportCluster: localClusterName,
			Expires:         now,
		}
		localSystemRole = tlsca.Identity{
			Username:        "node",
			Groups:          []string{string(types.RoleNode)},
			TeleportCluster: localClusterName,
			Expires:         now,
		}
		remoteUserIdentity = tlsca.Identity{
			Username:        "foo",
			Groups:          []string{"devs"},
			TeleportCluster: remoteClusterName,
			Expires:         now,
		}
		remoteSystemRole = tlsca.Identity{
			Username:        "node",
			Groups:          []string{string(types.RoleNode)},
			TeleportCluster: remoteClusterName,
			Expires:         now,
		}
	)

	localHostCA := suite.NewTestCA(types.HostCA, localClusterName)
	remoteHostCA := suite.NewTestCA(types.HostCA, remoteClusterName)
	localUserCA := suite.NewTestCA(types.UserCA, localClusterName)
	remoteUserCA := suite.NewTestCA(types.UserCA, remoteClusterName)

	caPool := buildPoolInfo(
		t,
		localHostCA,
		localUserCA,
		remoteHostCA,
		remoteUserCA,
	)

	tests := []struct {
		desc        string
		peer        *x509.Certificate
		clusterName string
		wantErr     bool
	}{
		{
			desc:        "local user issued from remote CA",
			peer:        generateTestCert(t, remoteUserCA, localUserIdentity, localClusterName),
			clusterName: localClusterName,
			wantErr:     true,
		},
		{
			desc:        "local system user issued from remote CA",
			peer:        generateTestCert(t, remoteHostCA, localSystemRole, localClusterName),
			clusterName: localClusterName,
			wantErr:     true,
		},
		{
			desc:        "local user issued from local host CA",
			peer:        generateTestCert(t, localHostCA, localUserIdentity, localClusterName),
			clusterName: localClusterName,
			wantErr:     true,
		},
		{
			desc:        "local system user issued from local user CA",
			peer:        generateTestCert(t, localUserCA, localSystemRole, localClusterName),
			clusterName: localClusterName,
			wantErr:     true,
		},
		{
			desc:        "local user with remote cluster name issued from local user CA",
			peer:        generateTestCert(t, localUserCA, localUserIdentity, remoteClusterName),
			clusterName: localClusterName,
			wantErr:     true,
		},
		{
			desc:        "local system user with remote cluster name issued from local host CA",
			peer:        generateTestCert(t, localHostCA, localSystemRole, remoteClusterName),
			clusterName: localClusterName,
			wantErr:     true,
		},
		{
			desc:        "local user  issued from local user CA",
			peer:        generateTestCert(t, localUserCA, localUserIdentity, localClusterName),
			clusterName: localClusterName,
			wantErr:     false,
		},
		{
			desc:        "local system user  issued from local host CA",
			peer:        generateTestCert(t, localHostCA, localSystemRole, localClusterName),
			clusterName: localClusterName,
			wantErr:     false,
		},
		{
			desc:        "remote user issued from local CA",
			peer:        generateTestCert(t, localUserCA, remoteUserIdentity, remoteClusterName),
			clusterName: remoteClusterName,
			wantErr:     true,
		},
		{
			desc:        "remote system user issued from local CA",
			peer:        generateTestCert(t, localHostCA, remoteSystemRole, remoteClusterName),
			clusterName: remoteClusterName,
			wantErr:     true,
		},
		{
			desc:        "remote user issued from remote host CA",
			peer:        generateTestCert(t, remoteHostCA, remoteUserIdentity, remoteClusterName),
			clusterName: remoteClusterName,
			wantErr:     true,
		},
		{
			desc:        "remote system user issued from user CA",
			peer:        generateTestCert(t, remoteUserCA, remoteSystemRole, remoteClusterName),
			clusterName: remoteClusterName,
			wantErr:     true,
		},
		{
			desc:        "remote user with local cluster name issued from remote user CA",
			peer:        generateTestCert(t, remoteUserCA, remoteUserIdentity, localClusterName),
			clusterName: remoteClusterName,
			wantErr:     true,
		},
		{
			desc:        "remote system user with local cluster name issued from host CA",
			peer:        generateTestCert(t, remoteHostCA, remoteSystemRole, localClusterName),
			clusterName: remoteClusterName,
			wantErr:     true,
		},
		{
			desc:        "remote user issued from remote user CA",
			peer:        generateTestCert(t, remoteUserCA, remoteUserIdentity, remoteClusterName),
			clusterName: remoteClusterName,
			wantErr:     false,
		},
		{
			desc:        "remote system user issued from host CA",
			peer:        generateTestCert(t, remoteHostCA, remoteSystemRole, remoteClusterName),
			clusterName: remoteClusterName,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			verify := caPool.verifyPeerCert()
			err := verify(nil, [][]*x509.Certificate{{tt.peer}})
			if tt.wantErr {
				require.ErrorContains(t, err, "access denied: invalid client certificate")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func buildPoolInfo(t *testing.T, ca ...types.CertAuthority) *HostAndUserCAPoolInfo {
	poolInfo := HostAndUserCAPoolInfo{
		Pool:    x509.NewCertPool(),
		CATypes: make(authclient.HostAndUserCAInfo),
	}

	for _, authority := range ca {
		for _, c := range authority.GetTrustedTLSKeyPairs() {
			cert, err := tlsca.ParseCertificatePEM(c.Cert)
			require.NoError(t, err)
			poolInfo.Pool.AddCert(cert)

			poolInfo.CATypes[string(cert.RawSubject)] = authclient.CATypeInfo{
				IsHostCA: authority.GetType() == types.HostCA,
				IsUserCA: authority.GetType() == types.UserCA,
			}
		}
	}

	return &poolInfo
}

func generateTestCert(t *testing.T, ca types.CertAuthority, id tlsca.Identity, clusterName string) *x509.Certificate {
	tlsKeyPairs := ca.GetTrustedTLSKeyPairs()
	require.Len(t, tlsKeyPairs, 1)
	signer, err := tlsca.FromKeys(tlsKeyPairs[0].Cert, tlsKeyPairs[0].Key)
	require.NoError(t, err)

	priv, err := testauthority.New().GeneratePrivateKey()
	require.NoError(t, err)

	id.TeleportCluster = clusterName
	subj, err := id.Subject()
	require.NoError(t, err)

	pemCert, err := signer.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: priv.Public(),
		Subject:   subj,
		NotAfter:  time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	block, rest := pem.Decode(pemCert)
	require.NotNil(t, block)
	require.Empty(t, rest)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	return cert
}
