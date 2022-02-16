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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto"
	"crypto/tls"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"gopkg.in/check.v1"

	"github.com/gravitational/trace"
)

type TLSSuite struct {
	dataDir string
	server  *TestTLSServer
	clock   clockwork.FakeClock
}

var _ = check.Suite(&TLSSuite{})

func (s *TLSSuite) SetUpTest(c *check.C) {
	s.dataDir = c.MkDir()
	s.clock = clockwork.NewFakeClock()

	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   s.dataDir,
		Clock: s.clock,
	})
	c.Assert(err, check.IsNil)
	s.server, err = testAuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
}

func (s *TLSSuite) TearDownTest(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
}

// TestRemoteBuiltinRole tests remote builtin role
// that gets mapped to remote proxy readonly role
func (s *TLSSuite) TestRemoteBuiltinRole(c *check.C) {
	ctx := context.Background()
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
		Clock:       s.clock,
	})
	c.Assert(err, check.IsNil)

	certPool, err := s.server.CertPool()
	c.Assert(err, check.IsNil)

	// without trust, proxy server will get rejected
	// remote auth server will get rejected because it is not supported
	remoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	// certificate authority is not recognized, because
	// the trust has not been established yet
	_, err = remoteProxy.GetNodes(ctx, apidefaults.Namespace)
	fixtures.ExpectConnectionProblem(c, err)

	// after trust is established, things are good
	err = s.server.AuthServer.Trust(remoteServer, nil)
	c.Assert(err, check.IsNil)

	// re initialize client with trust established.
	remoteProxy, err = remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	_, err = remoteProxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// remote auth server will get rejected even with established trust
	remoteAuth, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleAuth), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	_, err = remoteAuth.GetDomainName()
	fixtures.ExpectAccessDenied(c, err)
}

// TestAcceptedUsage tests scenario when server is set up
// to accept certificates with certain usage metadata restrictions
// encoded
func (s *TLSSuite) TestAcceptedUsage(c *check.C) {
	ctx := context.Background()
	server, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:           c.MkDir(),
		ClusterName:   "remote",
		AcceptedUsage: []string{"usage:k8s"},
		Clock:         s.clock,
	})
	c.Assert(err, check.IsNil)

	user, _, err := CreateUserAndRole(server.AuthServer, "user", []string{"role"})
	c.Assert(err, check.IsNil)

	tlsServer, err := server.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer tlsServer.Close()

	// Unrestricted clients can use restricted servers
	client, err := tlsServer.NewClient(TestUser(user.GetName()))
	c.Assert(err, check.IsNil)

	// certificate authority is not recognized, because
	// the trust has not been established yet
	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// restricted clients can use restricted servers if restrictions
	// exactly match
	identity := TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:k8s"}
	client, err = tlsServer.NewClient(identity)
	c.Assert(err, check.IsNil)

	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// restricted clients can will be rejected if usage does not match
	identity = TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:extra"}
	client, err = tlsServer.NewClient(identity)
	c.Assert(err, check.IsNil)

	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	fixtures.ExpectAccessDenied(c, err)

	// restricted clients can will be rejected, for now if there is any mismatch,
	// including extra usage.
	identity = TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:k8s", "usage:unknown"}
	client, err = tlsServer.NewClient(identity)
	c.Assert(err, check.IsNil)

	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	fixtures.ExpectAccessDenied(c, err)
}

// TestRemoteRotation tests remote builtin role
// that attempts certificate authority rotation
func (s *TLSSuite) TestRemoteRotation(c *check.C) {
	ctx := context.TODO()
	var ok bool

	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
		Clock:       s.clock,
	})
	c.Assert(err, check.IsNil)

	certPool, err := s.server.CertPool()
	c.Assert(err, check.IsNil)

	// after trust is established, things are good
	err = s.server.AuthServer.Trust(remoteServer, nil)
	c.Assert(err, check.IsNil)

	remoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	remoteAuth, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleAuth), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	// remote cluster starts rotation
	gracePeriod := time.Hour
	remoteServer.AuthServer.privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
	err = remoteServer.AuthServer.RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// moves to update clients
	err = remoteServer.AuthServer.RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	remoteCA, err := remoteServer.AuthServer.GetCertAuthority(types.CertAuthID{
		DomainName: remoteServer.ClusterName,
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	// remote proxy should be rejected when trying to rotate ca
	// that is not associated with the remote cluster
	clone := remoteCA.Clone()
	clone.SetName(s.server.ClusterName())
	err = remoteProxy.RotateExternalCertAuthority(clone)
	fixtures.ExpectAccessDenied(c, err)

	// remote proxy can't upsert the certificate authority,
	// only to rotate it (in remote rotation only certain fields are updated)
	err = remoteProxy.UpsertCertAuthority(remoteCA)
	fixtures.ExpectAccessDenied(c, err)

	// remote proxy can't read local cert authority with secrets
	_, err = remoteProxy.GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, true)
	fixtures.ExpectAccessDenied(c, err)

	// no secrets read is allowed
	_, err = remoteProxy.GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	// remote auth server will get rejected
	err = remoteAuth.RotateExternalCertAuthority(remoteCA)
	fixtures.ExpectAccessDenied(c, err)

	// remote proxy should be able to perform remote cert authority
	// rotation
	err = remoteProxy.RotateExternalCertAuthority(remoteCA)
	c.Assert(err, check.IsNil)

	// newRemoteProxy should be trusted by the auth server
	newRemoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(types.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	_, err = newRemoteProxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// old proxy client is still trusted
	_, err = s.server.CloneClient(remoteProxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)
}

// TestLocalProxyPermissions tests new local proxy permissions
// as it's now allowed to update host cert authorities of remote clusters
func (s *TLSSuite) TestLocalProxyPermissions(c *check.C) {
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
		Clock:       s.clock,
	})
	c.Assert(err, check.IsNil)

	// after trust is established, things are good
	err = s.server.AuthServer.Trust(remoteServer, nil)
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	// local proxy can't update local cert authorities
	err = proxy.UpsertCertAuthority(ca)
	fixtures.ExpectAccessDenied(c, err)

	// local proxy is allowed to update host CA of remote cert authorities
	remoteCA, err := s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: remoteServer.ClusterName,
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	err = proxy.UpsertCertAuthority(remoteCA)
	c.Assert(err, check.IsNil)
}

// TestAutoRotation tests local automatic rotation
func (s *TLSSuite) TestAutoRotation(c *check.C) {
	ctx := context.Background()
	var ok bool

	// create proxy client
	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// starts rotation
	s.server.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		Mode:        types.RotationModeAuto,
	})
	c.Assert(err, check.IsNil)

	// advance rotation by clock
	s.clock.Advance(gracePeriod/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, types.RotationPhaseUpdateClients)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// new clients work as well
	_, err = s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	// advance rotation by clock
	s.clock.Advance((gracePeriod*2)/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	c.Assert(err, check.IsNil)

	ca, err = s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, types.RotationPhaseUpdateServers)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// new clients work as well
	newProxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	_, err = newProxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// complete rotation - advance rotation by clock
	s.clock.Advance(gracePeriod/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	c.Assert(err, check.IsNil)
	ca, err = s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, types.RotationPhaseStandby)
	c.Assert(err, check.IsNil)

	// old clients should no longer work
	// new client has to be created here to force re-create the new
	// connection instead of re-using the one from pool
	// this is not going to be a problem in real teleport
	// as it reloads the full server after reload
	_, err = s.server.CloneClient(proxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// new clients work
	_, err = s.server.CloneClient(newProxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)
}

// TestAutoFallback tests local automatic rotation fallback,
// when user intervenes with rollback and rotation gets switched
// to manual mode
func (s *TLSSuite) TestAutoFallback(c *check.C) {
	ctx := context.Background()
	var ok bool

	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// starts rotation
	s.server.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		Mode:        types.RotationModeAuto,
	})
	c.Assert(err, check.IsNil)

	// advance rotation by clock
	s.clock.Advance(gracePeriod/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, types.RotationPhaseUpdateClients)
	c.Assert(ca.GetRotation().Mode, check.Equals, types.RotationModeAuto)

	// rollback rotation
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseRollback,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	ca, err = s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, types.RotationPhaseRollback)
	c.Assert(ca.GetRotation().Mode, check.Equals, types.RotationModeManual)
}

// TestManualRotation tests local manual rotation
// that performs full-cycle certificate authority rotation
func (s *TLSSuite) TestManualRotation(c *check.C) {
	ctx := context.Background()
	var ok bool

	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// can't jump to mid-phase
	gracePeriod := time.Hour
	s.server.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	fixtures.ExpectBadParameter(c, err)

	// starts rotation
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// clients reconnect
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// new clients work as well
	newProxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	_, err = newProxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// can't jump to standy
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	fixtures.ExpectBadParameter(c, err)

	// advance rotation:
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// new clients work as well
	_, err = s.server.CloneClient(newProxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// complete rotation
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// old clients should no longer work
	// new client has to be created here to force re-create the new
	// connection instead of re-using the one from pool
	// this is not going to be a problem in real teleport
	// as it reloads the full server after reload
	_, err = s.server.CloneClient(proxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// new clients work
	_, err = s.server.CloneClient(newProxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)
}

// TestRollback tests local manual rotation rollback
func (s *TLSSuite) TestRollback(c *check.C) {
	ctx := context.Background()
	var ok bool

	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// starts rotation
	gracePeriod := time.Hour
	s.server.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// move to update clients phase
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// new clients work
	newProxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	_, err = newProxy.GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// advance rotation:
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// rollback rotation
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseRollback,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// new clients work, server still accepts the creds
	// because new clients should re-register and receive new certs
	_, err = s.server.CloneClient(newProxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	// can't jump to other phases
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	fixtures.ExpectBadParameter(c, err)

	// complete rollback
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// clients with new creds will no longer work
	_, err = s.server.CloneClient(newProxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// clients with old creds will still work
	_, err = s.server.CloneClient(proxy).GetNodes(ctx, apidefaults.Namespace)
	c.Assert(err, check.IsNil)
}

// TestAppTokenRotation checks that JWT tokens can be rotated and tokens can or
// can not be validated at the appropriate phase.
func (s *TLSSuite) TestAppTokenRotation(c *check.C) {
	client, err := s.server.NewClient(TestBuiltin(types.RoleApp))
	c.Assert(err, check.IsNil)

	// Create a JWT using the current CA, this will become the "old" CA during
	// rotation.
	oldJWT, err := client.GenerateAppToken(context.Background(),
		types.GenerateAppTokenRequest{
			Username: "foo",
			Roles:    []string{"bar", "baz"},
			URI:      "http://localhost:8080",
			Expires:  s.clock.Now().Add(1 * time.Minute),
		})
	c.Assert(err, check.IsNil)

	// Check that the "old" CA can be used to verify tokens.
	oldCA, err := s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	c.Assert(err, check.IsNil)
	c.Assert(oldCA.GetTrustedJWTKeyPairs(), check.HasLen, 1)

	// Verify that the JWT token validates with the JWT authority.
	_, err = s.verifyJWT(s.clock, s.server.ClusterName(), oldCA.GetTrustedJWTKeyPairs(), oldJWT)
	c.Assert(err, check.IsNil)

	// Start rotation and move to initial phase. A new CA will be added (for
	// verification), but requests will continue to be signed by the old CA.
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.JWTSigner,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// At this point in rotation, two JWT key pairs should exist.
	oldCA, err = s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	c.Assert(err, check.IsNil)
	c.Assert(oldCA.GetRotation().Phase, check.Equals, types.RotationPhaseInit)
	c.Assert(oldCA.GetTrustedJWTKeyPairs(), check.HasLen, 2)

	// Verify that the JWT token validates with the JWT authority.
	_, err = s.verifyJWT(s.clock, s.server.ClusterName(), oldCA.GetTrustedJWTKeyPairs(), oldJWT)
	c.Assert(err, check.IsNil)

	// Move rotation into the update client phase. In this phase, requests will
	// be signed by the new CA, but the old CA will be around to verify requests.
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.JWTSigner,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// New tokens should now fail to validate with the old key.
	newJWT, err := client.GenerateAppToken(context.Background(),
		types.GenerateAppTokenRequest{
			Username: "foo",
			Roles:    []string{"bar", "baz"},
			URI:      "http://localhost:8080",
			Expires:  s.clock.Now().Add(1 * time.Minute),
		})
	c.Assert(err, check.IsNil)

	// New tokens will validate with the new key.
	newCA, err := s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	c.Assert(err, check.IsNil)
	c.Assert(newCA.GetRotation().Phase, check.Equals, types.RotationPhaseUpdateClients)
	c.Assert(newCA.GetTrustedJWTKeyPairs(), check.HasLen, 2)

	// Both JWT should now validate.
	_, err = s.verifyJWT(s.clock, s.server.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT)
	c.Assert(err, check.IsNil)
	_, err = s.verifyJWT(s.clock, s.server.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT)
	c.Assert(err, check.IsNil)

	// Move rotation into update servers phase.
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.JWTSigner,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// At this point only the phase on the CA should have changed.
	newCA, err = s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	c.Assert(err, check.IsNil)
	c.Assert(newCA.GetRotation().Phase, check.Equals, types.RotationPhaseUpdateServers)
	c.Assert(newCA.GetTrustedJWTKeyPairs(), check.HasLen, 2)

	// Both JWT should continue to validate.
	_, err = s.verifyJWT(s.clock, s.server.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT)
	c.Assert(err, check.IsNil)
	_, err = s.verifyJWT(s.clock, s.server.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT)
	c.Assert(err, check.IsNil)

	// Complete rotation. The old CA will be removed.
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.JWTSigner,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// The new CA should now only have a single key.
	newCA, err = s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.JWTSigner,
	}, true)
	c.Assert(err, check.IsNil)
	c.Assert(newCA.GetRotation().Phase, check.Equals, types.RotationPhaseStandby)
	c.Assert(newCA.GetTrustedJWTKeyPairs(), check.HasLen, 1)

	// Old token should no longer validate.
	_, err = s.verifyJWT(s.clock, s.server.ClusterName(), newCA.GetTrustedJWTKeyPairs(), oldJWT)
	c.Assert(err, check.NotNil)
	_, err = s.verifyJWT(s.clock, s.server.ClusterName(), newCA.GetTrustedJWTKeyPairs(), newJWT)
	c.Assert(err, check.IsNil)
}

// TestRemoteUser tests scenario when remote user connects to the local
// auth server and some edge cases.
func (s *TLSSuite) TestRemoteUser(c *check.C) {
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
		Clock:       s.clock,
	})
	c.Assert(err, check.IsNil)

	remoteUser, remoteRole, err := CreateUserAndRole(remoteServer.AuthServer, "remote-user", []string{"remote-role"})
	c.Assert(err, check.IsNil)

	certPool, err := s.server.CertPool()
	c.Assert(err, check.IsNil)

	remoteClient, err := remoteServer.NewRemoteClient(
		TestUser(remoteUser.GetName()), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	// User is not authorized to perform any actions
	// as local cluster does not trust the remote cluster yet
	_, err = remoteClient.GetDomainName()
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// Establish trust, the request will still fail, there is
	// no role mapping set up
	err = s.server.AuthServer.Trust(remoteServer, nil)
	c.Assert(err, check.IsNil)
	_, err = remoteClient.GetDomainName()
	fixtures.ExpectAccessDenied(c, err)

	// Establish trust and map remote role to local admin role
	_, localRole, err := CreateUserAndRole(s.server.Auth(), "local-user", []string{"local-role"})
	c.Assert(err, check.IsNil)

	err = s.server.AuthServer.Trust(remoteServer, types.RoleMap{{Remote: remoteRole.GetName(), Local: []string{localRole.GetName()}}})
	c.Assert(err, check.IsNil)

	_, err = remoteClient.GetDomainName()
	c.Assert(err, check.IsNil)
}

// TestNopUser tests user with no permissions except
// the ones that require other authentication methods ("nop" user)
func (s *TLSSuite) TestNopUser(c *check.C) {
	ctx := context.Background()
	client, err := s.server.NewClient(TestNop())
	c.Assert(err, check.IsNil)

	// Nop User can get cluster name
	_, err = client.GetDomainName()
	c.Assert(err, check.IsNil)

	// But can not get users or nodes
	_, err = client.GetUsers(false)
	fixtures.ExpectAccessDenied(c, err)

	_, err = client.GetNodes(ctx, apidefaults.Namespace)
	fixtures.ExpectAccessDenied(c, err)

	// Endpoints that allow current user access should return access denied to
	// the Nop user.
	err = client.CheckPassword("foo", nil, "")
	fixtures.ExpectAccessDenied(c, err)
}

// TestOwnRole tests that user can read roles assigned to them (used by web UI)
func (s *TLSSuite) TestReadOwnRole(c *check.C) {
	ctx := context.Background()

	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user1, userRole, err := CreateUserAndRoleWithoutRoles(clt, "user1", []string{"user1"})
	c.Assert(err, check.IsNil)

	user2, _, err := CreateUserAndRoleWithoutRoles(clt, "user2", []string{"user2"})
	c.Assert(err, check.IsNil)

	// user should be able to read their own roles
	userClient, err := s.server.NewClient(TestUser(user1.GetName()))
	c.Assert(err, check.IsNil)

	_, err = userClient.GetRole(ctx, userRole.GetName())
	c.Assert(err, check.IsNil)

	// user2 can't read user1 role
	userClient2, err := s.server.NewClient(TestIdentity{I: LocalUser{Username: user2.GetName()}})
	c.Assert(err, check.IsNil)

	_, err = userClient2.GetRole(ctx, userRole.GetName())
	fixtures.ExpectAccessDenied(c, err)
}

func (s *TLSSuite) TestAuthPreference(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		ConfigS: clt,
	}
	suite.AuthPreference(c)
}

func (s *TLSSuite) TestTunnelConnectionsCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
		Clock:     clockwork.NewFakeClock(),
	}
	suite.TunnelConnectionsCRUD(c)
}

func (s *TLSSuite) TestRemoteClustersCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.RemoteClustersCRUD(c)
}

func (s *TLSSuite) TestServersCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.ServerCRUD(c)
}

// TestAppServerCRUD tests CRUD functionality for services.App using an auth client.
func (s *TLSSuite) TestAppServerCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestBuiltin(types.RoleApp))
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.AppServerCRUD(c)
}

func (s *TLSSuite) TestReverseTunnelsCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
	}
	suite.ReverseTunnelsCRUD(c)
}

func (s *TLSSuite) TestUsersCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	err = clt.UpsertPassword("user1", []byte("some pass"))
	c.Assert(err, check.IsNil)

	users, err := clt.GetUsers(false)
	c.Assert(err, check.IsNil)
	c.Assert(len(users), check.Equals, 1)
	c.Assert(users[0].GetName(), check.Equals, "user1")

	c.Assert(clt.DeleteUser(context.TODO(), "user1"), check.IsNil)

	users, err = clt.GetUsers(false)
	c.Assert(err, check.IsNil)
	c.Assert(len(users), check.Equals, 0)
}

func (s *TLSSuite) TestPasswordGarbage(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)
	garbage := [][]byte{
		nil,
		make([]byte, defaults.MaxPasswordLength+1),
		make([]byte, defaults.MinPasswordLength-1),
	}
	for _, g := range garbage {
		err := clt.CheckPassword("user1", g, "123456")
		fixtures.ExpectBadParameter(c, err)
	}
}

func (s *TLSSuite) TestPasswordCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	pass := []byte("abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	err = clt.CheckPassword("user1", pass, "123456")
	c.Assert(err, check.NotNil)

	err = clt.UpsertPassword("user1", pass)
	c.Assert(err, check.IsNil)

	dev, err := services.NewTOTPDevice("otp", otpSecret, s.clock.Now())
	c.Assert(err, check.IsNil)
	ctx := context.Background()
	err = s.server.Auth().UpsertMFADevice(ctx, "user1", dev)
	c.Assert(err, check.IsNil)

	validToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	err = clt.CheckPassword("user1", pass, validToken)
	c.Assert(err, check.IsNil)
}

func (s *TLSSuite) TestTokens(c *check.C) {
	ctx := context.Background()
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	out, err := clt.GenerateToken(ctx, GenerateTokenRequest{Roles: types.SystemRoles{types.RoleNode}})
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Not(check.Equals), 0)
}

func (s *TLSSuite) TestValidateUploadSessionRecording(c *check.C) {
	serverID, err := s.server.Identity.ID.HostID()
	c.Assert(err, check.IsNil)

	tests := []struct {
		inServerID string
		outError   bool
	}{
		// Invalid.
		{
			inServerID: "00000000-0000-0000-0000-000000000000",
			outError:   true,
		},
		// Valid.
		{
			inServerID: serverID,
			outError:   false,
		},
	}
	for _, tt := range tests {
		clt, err := s.server.NewClient(TestServerID(types.RoleNode, serverID))
		c.Assert(err, check.IsNil)

		sessionID := session.NewID()

		recording, err := makeSessionRecording(sessionID.String(), tt.inServerID)
		c.Assert(err, check.IsNil)

		date := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
		sess := session.Session{
			ID:             sessionID,
			TerminalParams: session.TerminalParams{W: 100, H: 100},
			Created:        date,
			LastActive:     date,
			Login:          "bob",
			Namespace:      apidefaults.Namespace,
		}
		c.Assert(clt.CreateSession(sess), check.IsNil)

		err = clt.UploadSessionRecording(events.SessionRecording{
			Namespace: apidefaults.Namespace,
			SessionID: sess.ID,
			Recording: recording,
		})
		c.Assert(err != nil, check.Equals, tt.outError)
	}
}

func makeSessionRecording(sessionID string, serverID string) (io.Reader, error) {
	marshal := func(f events.EventFields) []byte {
		data, err := json.Marshal(f)
		if err != nil {
			panic(err)
		}
		return data
	}

	var zbuf bytes.Buffer
	zw := gzip.NewWriter(&zbuf)

	zw.Name = fmt.Sprintf("%v-0.events", sessionID)
	_, err := zw.Write(marshal(events.EventFields{
		events.SessionServerID: serverID,
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = zw.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tbuf bytes.Buffer
	tw := tar.NewWriter(&tbuf)

	hdr := &tar.Header{
		Name: fmt.Sprintf("%v-0.events.gz", sessionID),
		Mode: 0600,
		Size: int64(zbuf.Len()),
	}
	err = tw.WriteHeader(hdr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = tw.Write(zbuf.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = tw.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &tbuf, nil
}

func (s *TLSSuite) TestValidatePostSessionSlice(c *check.C) {
	serverID, err := s.server.Identity.ID.HostID()
	c.Assert(err, check.IsNil)

	tests := []struct {
		inServerID string
		outError   bool
	}{
		// Invalid.
		{
			inServerID: "00000000-0000-0000-0000-000000000000",
			outError:   true,
		},
		// Valid.
		{
			inServerID: serverID,
			outError:   false,
		},
	}
	for _, tt := range tests {
		clt, err := s.server.NewClient(TestServerID(types.RoleNode, serverID))
		c.Assert(err, check.IsNil)

		date := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
		sess := session.Session{
			ID:             session.NewID(),
			TerminalParams: session.TerminalParams{W: 100, H: 100},
			Created:        date,
			LastActive:     date,
			Login:          "bob",
			Namespace:      apidefaults.Namespace,
		}
		c.Assert(clt.CreateSession(sess), check.IsNil)

		marshal := func(f events.EventFields) []byte {
			data, err := json.Marshal(f)
			if err != nil {
				panic(err)
			}
			return data
		}

		err = clt.PostSessionSlice(events.SessionSlice{
			Namespace: apidefaults.Namespace,
			SessionID: string(sess.ID),
			Chunks: []*events.SessionChunk{
				{
					Time:       time.Now().UTC().UnixNano(),
					EventIndex: 0,
					EventType:  events.SessionStartEvent,
					Data: marshal(events.EventFields{
						events.SessionServerID: tt.inServerID,
					}),
				},
			},
			Version: events.V3,
		})
		c.Assert(err != nil, check.Equals, tt.outError)
	}
}

func (s *TLSSuite) TestSharedSessions(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	out, err := clt.GetSessions(apidefaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.DeepEquals, []session.Session{})

	date := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	sess := session.Session{
		ID:             session.NewID(),
		TerminalParams: session.TerminalParams{W: 100, H: 100},
		Created:        date,
		LastActive:     date,
		Login:          "bob",
		Namespace:      apidefaults.Namespace,
	}
	c.Assert(clt.CreateSession(sess), check.IsNil)

	out, err = clt.GetSessions(apidefaults.Namespace)
	c.Assert(err, check.IsNil)

	c.Assert(out, check.DeepEquals, []session.Session{sess})
	marshal := func(f events.EventFields) []byte {
		data, err := json.Marshal(f)
		if err != nil {
			panic(err)
		}
		return data
	}

	uploadDir := c.MkDir()

	// emit two events: "one" and "two" for this session, and event "three"
	// for some other session
	err = os.MkdirAll(filepath.Join(uploadDir, "upload", "sessions", apidefaults.Namespace), 0755)
	c.Assert(err, check.IsNil)
	forwarder, err := events.NewForwarder(events.ForwarderConfig{
		Namespace:      apidefaults.Namespace,
		SessionID:      sess.ID,
		ServerID:       teleport.ComponentUpload,
		DataDir:        uploadDir,
		RecordSessions: true,
		IAuditLog:      clt,
	})
	c.Assert(err, check.IsNil)

	err = forwarder.PostSessionSlice(events.SessionSlice{
		Namespace: apidefaults.Namespace,
		SessionID: string(sess.ID),
		Chunks: []*events.SessionChunk{
			{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  events.SessionStartEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "bob", "val": "one"}),
			},
			{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 1,
				EventType:  events.SessionEndEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "bob", "val": "two"}),
			},
		},
		Version: events.V3,
	})
	c.Assert(err, check.IsNil)
	c.Assert(forwarder.Close(), check.IsNil)

	anotherSessionID := session.NewID()
	forwarder, err = events.NewForwarder(events.ForwarderConfig{
		Namespace:      apidefaults.Namespace,
		SessionID:      sess.ID,
		ServerID:       teleport.ComponentUpload,
		DataDir:        uploadDir,
		RecordSessions: true,
		IAuditLog:      clt,
	})
	c.Assert(err, check.IsNil)
	err = clt.PostSessionSlice(events.SessionSlice{
		Namespace: apidefaults.Namespace,
		SessionID: string(anotherSessionID),
		Chunks: []*events.SessionChunk{
			{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  events.SessionStartEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "alice"}),
			},
			{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 1,
				EventType:  events.SessionEndEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "alice"}),
			},
		},
		Version: events.V3,
	})
	c.Assert(err, check.IsNil)
	c.Assert(forwarder.Close(), check.IsNil)

	// start uploader process
	eventsC := make(chan events.UploadEvent, 100)
	uploader, err := events.NewUploader(events.UploaderConfig{
		ServerID:   "upload",
		DataDir:    uploadDir,
		Namespace:  apidefaults.Namespace,
		Context:    context.TODO(),
		ScanPeriod: 100 * time.Millisecond,
		AuditLog:   clt,
		EventsC:    eventsC,
	})
	c.Assert(err, check.IsNil)
	err = uploader.Scan()
	c.Assert(err, check.IsNil)

	// scanner should upload the events
	select {
	case event := <-eventsC:
		c.Assert(event, check.NotNil)
		c.Assert(event.Error, check.IsNil)
	case <-time.After(time.Second):
		c.Fatalf("Timeout wating for the upload event")
	}

	// ask for strictly session events:
	e, err := clt.GetSessionEvents(apidefaults.Namespace, sess.ID, 0, true)
	c.Assert(err, check.IsNil)
	c.Assert(len(e), check.Equals, 2)
	c.Assert(e[0].GetString("val"), check.Equals, "one")
	c.Assert(e[1].GetString("val"), check.Equals, "two")

	// try searching for events with no filter (empty query) - should get all 3 events:
	to := time.Now().In(time.UTC).Add(time.Hour)
	from := to.Add(-time.Hour * 2)
	history, _, err := clt.SearchEvents(from, to, apidefaults.Namespace, nil, 0, types.EventOrderDescending, "")
	c.Assert(err, check.IsNil)
	c.Assert(history, check.NotNil)
	// Extra event is the upload event
	c.Assert(len(history), check.Equals, 5)

	// try searching for only "session.end" events (real query)
	history, _, err = clt.SearchEvents(from, to, apidefaults.Namespace, []string{events.SessionEndEvent}, 0, types.EventOrderDescending, "")
	c.Assert(err, check.IsNil)
	c.Assert(history, check.NotNil)
	c.Assert(len(history), check.Equals, 2)
	var found bool
	for _, event := range history {
		realEvent, ok := event.(*apievents.SessionEnd)
		c.Assert(ok, check.Equals, true)
		if realEvent.GetSessionID() == string(anotherSessionID) {
			found = true
			c.Assert(realEvent.Login, check.Equals, "alice")
		}
	}
	c.Assert(found, check.Equals, true)
}

func (s *TLSSuite) TestOTPCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "user1"
	pass := []byte("abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	// upsert a password and totp secret
	err = clt.UpsertPassword("user1", pass)
	c.Assert(err, check.IsNil)
	dev, err := services.NewTOTPDevice("otp", otpSecret, s.clock.Now())
	c.Assert(err, check.IsNil)
	ctx := context.Background()
	err = s.server.Auth().UpsertMFADevice(ctx, user, dev)
	c.Assert(err, check.IsNil)

	// a completely invalid token should return access denied
	err = clt.CheckPassword("user1", pass, "123456")
	c.Assert(err, check.NotNil)

	// an invalid token should return access denied
	//
	// this tests makes the token 61 seconds in the future (but from a valid key)
	// even though the validity period is 30 seconds. this is because a token is
	// valid for 30 seconds + 30 second skew before and after for a usability
	// reasons. so a token made between seconds 31 and 60 is still valid, and
	// invalidity starts at 61 seconds in the future.
	invalidToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now().Add(61*time.Second))
	c.Assert(err, check.IsNil)
	err = clt.CheckPassword("user1", pass, invalidToken)
	c.Assert(err, check.NotNil)

	// a valid token (created right now and from a valid key) should return success
	validToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	err = clt.CheckPassword("user1", pass, validToken)
	c.Assert(err, check.IsNil)

	// try the same valid token now it should fail because we don't allow re-use of tokens
	err = clt.CheckPassword("user1", pass, validToken)
	c.Assert(err, check.NotNil)
}

// TestWebSessions tests web sessions flow for web user,
// that logs in, extends web session and tries to perform administratvie action
// but fails
func (s *TLSSuite) TestWebSessionWithoutAccessRequest(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "user1"
	pass := []byte("abc123")

	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	req := AuthenticateUserRequest{
		Username: user,
		Pass: &PassCreds{
			Password: pass,
		},
	}
	// authentication attempt fails with no password set up
	_, err = proxy.AuthenticateWebUser(req)
	fixtures.ExpectAccessDenied(c, err)

	err = clt.UpsertPassword(user, pass)
	c.Assert(err, check.IsNil)

	// success with password set up
	ws, err := proxy.AuthenticateWebUser(req)
	c.Assert(err, check.IsNil)
	c.Assert(ws, check.Not(check.Equals), "")

	web, err := s.server.NewClientFromWebSession(ws)
	c.Assert(err, check.IsNil)

	_, err = web.GetWebSessionInfo(context.TODO(), user, ws.GetName())
	c.Assert(err, check.IsNil)

	new, err := web.ExtendWebSession(WebSessionReq{
		User:          user,
		PrevSessionID: ws.GetName(),
	})
	c.Assert(err, check.IsNil)
	c.Assert(new, check.NotNil)

	// Requesting forbidden action for user fails
	err = web.DeleteUser(context.TODO(), user)
	fixtures.ExpectAccessDenied(c, err)

	err = clt.DeleteWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)

	_, err = web.GetWebSessionInfo(context.TODO(), user, ws.GetName())
	c.Assert(err, check.NotNil)

	_, err = web.ExtendWebSession(WebSessionReq{
		User:          user,
		PrevSessionID: ws.GetName(),
	})
	c.Assert(err, check.NotNil)
}

func (s *TLSSuite) TestWebSessionWithApprovedAccessRequestAndSwitchback(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "user2"
	pass := []byte("abc123")

	newUser, err := CreateUserRoleAndRequestable(clt, user, "test-request-role")
	c.Assert(err, check.IsNil)
	c.Assert(newUser.GetRoles(), check.HasLen, 1)
	c.Assert(newUser.GetRoles(), check.DeepEquals, []string{"user:user2"})

	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	// Create a user to create a web session for.
	req := AuthenticateUserRequest{
		Username: user,
		Pass: &PassCreds{
			Password: pass,
		},
	}

	err = clt.UpsertPassword(user, pass)
	c.Assert(err, check.IsNil)

	ws, err := proxy.AuthenticateWebUser(req)
	c.Assert(err, check.IsNil)

	web, err := s.server.NewClientFromWebSession(ws)
	c.Assert(err, check.IsNil)

	initialRole := newUser.GetRoles()[0]
	initialSession, err := web.GetWebSessionInfo(context.TODO(), user, ws.GetName())
	c.Assert(err, check.IsNil)

	// Create a approved access request.
	accessReq, err := services.NewAccessRequest(user, []string{"test-request-role"}...)
	c.Assert(err, check.IsNil)

	// Set a lesser expiry date, to test switching back to default expiration later.
	accessReq.SetAccessExpiry(s.clock.Now().Add(time.Minute * 10))
	accessReq.SetState(types.RequestState_APPROVED)

	err = clt.CreateAccessRequest(context.Background(), accessReq)
	c.Assert(err, check.IsNil)

	sess1, err := web.ExtendWebSession(WebSessionReq{
		User:            user,
		PrevSessionID:   ws.GetName(),
		AccessRequestID: accessReq.GetMetadata().Name,
	})
	c.Assert(err, check.IsNil)
	c.Assert(sess1.Expiry(), check.Equals, s.clock.Now().Add(time.Minute*10))
	c.Assert(sess1.GetLoginTime(), check.Equals, initialSession.GetLoginTime())

	sshcert, err := sshutils.ParseCertificate(sess1.GetPub())
	c.Assert(err, check.IsNil)

	// Roles extracted from cert should contain the initial role and the role assigned with access request.
	roles, _, err := services.ExtractFromCertificate(sshcert)
	c.Assert(err, check.IsNil)
	c.Assert(roles, check.HasLen, 2)

	mappedRole := map[string]string{
		roles[0]: "",
		roles[1]: "",
	}

	_, hasRole := mappedRole[initialRole]
	c.Assert(hasRole, check.Equals, true)

	_, hasRole = mappedRole["test-request-role"]
	c.Assert(hasRole, check.Equals, true)

	// certRequests extracts the active requests from a PEM encoded TLS cert.
	certRequests := func(tlsCert []byte) []string {
		cert, err := tlsca.ParseCertificatePEM(tlsCert)
		c.Assert(err, check.IsNil)

		identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		c.Assert(err, check.IsNil)

		return identity.ActiveRequests
	}

	c.Assert(certRequests(sess1.GetTLSCert()), check.DeepEquals, []string{accessReq.GetName()})

	// Test switch back to default role and expiry.
	sess2, err := web.ExtendWebSession(WebSessionReq{
		User:          user,
		PrevSessionID: ws.GetName(),
		Switchback:    true,
	})
	c.Assert(err, check.IsNil)
	c.Assert(sess2.GetExpiryTime(), check.Equals, initialSession.GetExpiryTime())
	c.Assert(sess2.GetLoginTime(), check.Equals, initialSession.GetLoginTime())

	sshcert, err = sshutils.ParseCertificate(sess2.GetPub())
	c.Assert(err, check.IsNil)

	roles, _, err = services.ExtractFromCertificate(sshcert)
	c.Assert(err, check.IsNil)
	c.Assert(roles, check.DeepEquals, []string{initialRole})

	c.Assert(certRequests(sess2.GetTLSCert()), check.HasLen, 0)
}

// TestGetCertAuthority tests certificate authority permissions
func (s *TLSSuite) TestGetCertAuthority(c *check.C) {
	ctx := context.Background()
	// generate server keys for node
	nodeClt, err := s.server.NewClient(TestIdentity{I: BuiltinRole{Username: "00000000-0000-0000-0000-000000000000", Role: types.RoleNode}})
	c.Assert(err, check.IsNil)
	defer nodeClt.Close()

	// node is authorized to fetch CA without secrets
	ca, err := nodeClt.GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	for _, keyPair := range ca.GetActiveKeys().TLS {
		c.Assert(keyPair.Key, check.IsNil)
	}
	for _, keyPair := range ca.GetActiveKeys().SSH {
		c.Assert(keyPair.PrivateKey, check.IsNil)
	}

	// node is not authorized to fetch CA with secrets
	_, err = nodeClt.GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, true)
	fixtures.ExpectAccessDenied(c, err)

	// non-admin users are not allowed to get access to private key material
	user, err := types.NewUser("bob")
	c.Assert(err, check.IsNil)

	role := services.RoleForUser(user)
	role.SetLogins(types.Allow, []string{user.GetName()})
	err = s.server.Auth().UpsertRole(ctx, role)
	c.Assert(err, check.IsNil)

	user.AddRole(role.GetName())
	err = s.server.Auth().UpsertUser(user)
	c.Assert(err, check.IsNil)

	userClt, err := s.server.NewClient(TestUser(user.GetName()))
	c.Assert(err, check.IsNil)
	defer userClt.Close()

	// user is authorized to fetch CA without secrets
	_, err = userClt.GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	// user is not authorized to fetch CA with secrets
	_, err = userClt.GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, true)
	fixtures.ExpectAccessDenied(c, err)
}

func (s *TLSSuite) TestAccessRequest(c *check.C) {
	priv, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// make sure we can parse the private and public key
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	c.Assert(err, check.IsNil)

	_, err = tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	c.Assert(err, check.IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, check.IsNil)

	// create a user with one requestable role
	user := "user1"
	role := "some-role"
	_, err = CreateUserRoleAndRequestable(s.server.Auth(), user, role)
	c.Assert(err, check.IsNil)

	testUser := TestUser(user)
	testUser.TTL = time.Hour
	userClient, err := s.server.NewClient(testUser)
	c.Assert(err, check.IsNil)

	// Verify that user has correct requestable roles
	caps, err := userClient.GetAccessCapabilities(context.TODO(), types.AccessCapabilitiesRequest{
		RequestableRoles: true,
	})
	c.Assert(err, check.IsNil)
	c.Assert(caps.RequestableRoles, check.DeepEquals, []string{role})

	// create a user with no requestable roles
	user2, _, err := CreateUserAndRole(s.server.Auth(), "user2", []string{"user2"})
	c.Assert(err, check.IsNil)

	testUser2 := TestUser(user2.GetName())
	testUser2.TTL = time.Hour
	userClient2, err := s.server.NewClient(testUser2)
	c.Assert(err, check.IsNil)

	// verify that no requestable roles are shown for user2
	caps2, err := userClient2.GetAccessCapabilities(context.TODO(), types.AccessCapabilitiesRequest{
		RequestableRoles: true,
	})
	c.Assert(err, check.IsNil)
	c.Assert(caps2.RequestableRoles, check.HasLen, 0)

	// create an allowable access request for user1
	req, err := services.NewAccessRequest(user, role)
	c.Assert(err, check.IsNil)

	c.Assert(userClient.CreateAccessRequest(context.TODO(), req), check.IsNil)

	// sanity check; ensure that roles for which no `allow` directive
	// exists cannot be requested.
	badReq, err := services.NewAccessRequest(user, "some-fake-role")
	c.Assert(err, check.IsNil)
	c.Assert(userClient.CreateAccessRequest(context.TODO(), badReq), check.NotNil)

	// generateCerts executes a GenerateUserCerts request, optionally applying
	// one or more access-requests to the certificate.
	generateCerts := func(reqIDs ...string) (*proto.Certs, error) {
		return userClient.GenerateUserCerts(context.TODO(), proto.UserCertsRequest{
			PublicKey:      pub,
			Username:       user,
			Expires:        time.Now().Add(time.Hour).UTC(),
			Format:         constants.CertificateFormatStandard,
			AccessRequests: reqIDs,
		})
	}

	// certContainsRole checks if a PEM encoded TLS cert contains the
	// specified role.
	certContainsRole := func(tlsCert []byte, role string) bool {
		cert, err := tlsca.ParseCertificatePEM(tlsCert)
		c.Assert(err, check.IsNil)

		identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		c.Assert(err, check.IsNil)

		return apiutils.SliceContainsStr(identity.Groups, role)
	}

	// certRequests extracts the active requests from a PEM encoded TLS cert.
	certRequests := func(tlsCert []byte) []string {
		cert, err := tlsca.ParseCertificatePEM(tlsCert)
		c.Assert(err, check.IsNil)

		identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		c.Assert(err, check.IsNil)

		return identity.ActiveRequests
	}

	// certLogins extracts the logins from an ssh certificate
	certLogins := func(sshCert []byte) []string {
		cert, err := sshutils.ParseCertificate(sshCert)
		c.Assert(err, check.IsNil)
		return cert.ValidPrincipals
	}

	// sanity check; ensure that role is not held if no request is applied.
	userCerts, err := generateCerts()
	c.Assert(err, check.IsNil)
	if certContainsRole(userCerts.TLS, role) {
		c.Errorf("unexpected role %s", role)
	}
	// ensure that the default identity doesn't have any active requests
	c.Assert(certRequests(userCerts.TLS), check.HasLen, 0)

	// verify that cert for user with no static logins is generated with
	// exactly one login and that it is an invalid unix login (indicated
	// by preceding dash (-).
	logins := certLogins(userCerts.SSH)
	c.Assert(len(logins), check.Equals, 1)
	c.Assert(rune(logins[0][0]), check.Equals, '-')

	// attempt to apply request in PENDING state (should fail)
	_, err = generateCerts(req.GetName())
	c.Assert(err, check.NotNil)

	ctx := context.Background()

	// verify that user does not have the ability to approve their own request (not a special case, this
	// user just wasn't created with the necessary roles for request management).
	c.Assert(userClient.SetAccessRequestState(ctx, types.AccessRequestUpdate{RequestID: req.GetName(), State: types.RequestState_APPROVED}), check.NotNil)

	// attempt to apply request in APPROVED state (should succeed)
	c.Assert(s.server.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{RequestID: req.GetName(), State: types.RequestState_APPROVED}), check.IsNil)
	userCerts, err = generateCerts(req.GetName())
	c.Assert(err, check.IsNil)
	// ensure that the requested role was actually applied to the cert
	if !certContainsRole(userCerts.TLS, role) {
		c.Errorf("missing requested role %s", role)
	}
	// ensure that the request is stored in the certs
	c.Assert(certRequests(userCerts.TLS), check.DeepEquals, []string{req.GetName()})

	// verify that dynamically applied role granted a login,
	// which is is valid and has replaced the dummy login.
	logins = certLogins(userCerts.SSH)
	c.Assert(len(logins), check.Equals, 1)
	c.Assert(rune(logins[0][0]), check.Not(check.Equals), '-')

	elevatedCert, err := tls.X509KeyPair(userCerts.TLS, priv)
	c.Assert(err, check.IsNil)
	elevatedClient := s.server.NewClientWithCert(elevatedCert)

	newCerts, err := elevatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: pub,
		Username:  user,
		Expires:   time.Now().Add(time.Hour).UTC(),
		Format:    constants.CertificateFormatStandard,
		// no new access requests
		AccessRequests: nil,
	})
	c.Assert(err, check.IsNil)

	// in spite of having no access requests, we still have elevated roles...
	if !certContainsRole(newCerts.TLS, role) {
		c.Errorf("missing requested role %s", role)
	}
	// ...and the certificate shows the access request
	c.Assert(certRequests(newCerts.TLS), check.DeepEquals, []string{req.GetName()})

	// attempt to apply request in DENIED state (should fail)
	c.Assert(s.server.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{RequestID: req.GetName(), State: types.RequestState_DENIED}), check.IsNil)
	_, err = generateCerts(req.GetName())
	c.Assert(err, check.NotNil)

	// ensure that once in the DENIED state, a request cannot be set back to PENDING state.
	c.Assert(s.server.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{RequestID: req.GetName(), State: types.RequestState_PENDING}), check.NotNil)

	// ensure that once in the DENIED state, a request cannot be set back to APPROVED state.
	c.Assert(s.server.Auth().SetAccessRequestState(ctx, types.AccessRequestUpdate{RequestID: req.GetName(), State: types.RequestState_APPROVED}), check.NotNil)

	// ensure that identities with requests in the DENIED state can't reissue new certs.
	_, err = elevatedClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: pub,
		Username:  user,
		Expires:   time.Now().Add(time.Hour).UTC(),
		Format:    constants.CertificateFormatStandard,
		// no new access requests
		AccessRequests: nil,
	})
	c.Assert(err, check.NotNil)
}

func (s *TLSSuite) TestPluginData(c *check.C) {
	priv, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// make sure we can parse the private and public key
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	c.Assert(err, check.IsNil)

	_, err = tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	c.Assert(err, check.IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, check.IsNil)

	user := "user1"
	role := "some-role"
	_, err = CreateUserRoleAndRequestable(s.server.Auth(), user, role)
	c.Assert(err, check.IsNil)

	testUser := TestUser(user)
	testUser.TTL = time.Hour
	userClient, err := s.server.NewClient(testUser)
	c.Assert(err, check.IsNil)

	plugin := "my-plugin"
	_, err = CreateAccessPluginUser(context.TODO(), s.server.Auth(), plugin)
	c.Assert(err, check.IsNil)

	pluginUser := TestUser(plugin)
	pluginUser.TTL = time.Hour
	pluginClient, err := s.server.NewClient(pluginUser)
	c.Assert(err, check.IsNil)

	req, err := services.NewAccessRequest(user, role)
	c.Assert(err, check.IsNil)

	c.Assert(userClient.CreateAccessRequest(context.TODO(), req), check.IsNil)

	err = pluginClient.UpdatePluginData(context.TODO(), types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: req.GetName(),
		Plugin:   plugin,
		Set: map[string]string{
			"foo": "bar",
		},
	})
	c.Assert(err, check.IsNil)

	data, err := pluginClient.GetPluginData(context.TODO(), types.PluginDataFilter{
		Kind:     types.KindAccessRequest,
		Resource: req.GetName(),
	})
	c.Assert(err, check.IsNil)
	c.Assert(len(data), check.Equals, 1)

	entry, ok := data[0].Entries()[plugin]
	c.Assert(ok, check.Equals, true)
	c.Assert(entry.Data, check.DeepEquals, map[string]string{"foo": "bar"})

	err = pluginClient.UpdatePluginData(context.TODO(), types.PluginDataUpdateParams{
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
	c.Assert(err, check.IsNil)

	data, err = pluginClient.GetPluginData(context.TODO(), types.PluginDataFilter{
		Kind:     types.KindAccessRequest,
		Resource: req.GetName(),
	})
	c.Assert(err, check.IsNil)
	c.Assert(len(data), check.Equals, 1)

	entry, ok = data[0].Entries()[plugin]
	c.Assert(ok, check.Equals, true)
	c.Assert(entry.Data, check.DeepEquals, map[string]string{"spam": "eggs"})
}

// TestGenerateCerts tests edge cases around authorization of
// certificate generation for servers and users
func TestGenerateCerts(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)
	priv, pub, err := srv.Auth().GenerateKeyPair("")
	require.NoError(t, err)

	// make sure we can parse the private and public key
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	require.NoError(t, err)

	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	require.NoError(t, err)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)

	// generate server keys for node
	hostID := "00000000-0000-0000-0000-000000000000"
	hostClient, err := srv.NewClient(TestIdentity{I: BuiltinRole{Username: hostID, Role: types.RoleNode}})
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
	hostClient, err = srv.NewClient(TestIdentity{I: BuiltinRole{Username: hostID, Role: types.RoleNode}})
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

	user1, userRole, err := CreateUserAndRole(srv.Auth(), "user1", []string{"user1"})
	require.NoError(t, err)

	user2, userRole2, err := CreateUserAndRole(srv.Auth(), "user2", []string{"user2"})
	require.NoError(t, err)

	t.Run("Nop", func(t *testing.T) {
		// unauthenticated client should NOT be able to generate a user cert without auth
		nopClient, err := srv.NewClient(TestNop())
		require.NoError(t, err)

		_, err = nopClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   time.Now().Add(time.Hour).UTC(),
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
			Expires:   time.Now().Add(time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	parseCert := func(sshCert []byte) (*ssh.Certificate, time.Duration) {
		parsedCert, err := sshutils.ParseCertificate(sshCert)
		require.NoError(t, err)
		validBefore := time.Unix(int64(parsedCert.ValidBefore), 0)
		return parsedCert, time.Until(validBefore)
	}

	clock := srv.Auth().GetClock()
	t.Run("ImpersonateAllow", func(t *testing.T) {
		// Super impersonator impersonate anyone and login as root
		maxSessionTTL := 300 * time.Hour
		superImpersonatorRole, err := types.NewRole("superimpersonator", types.RoleSpecV5{
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
		role, err := types.NewRole("impersonate", types.RoleSpecV5{
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
		require.Less(t, int64(diff), int64(iUser.TTL))

		tlsCert, err := tlsca.ParseCertificatePEM(userCerts.TLS)
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)

		// Because the original request has maxed out the possible max
		// session TTL, it will be adjusted to exactly the value
		require.Equal(t, identity.Expires.Sub(clock.Now()), maxSessionTTL)
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
		require.IsType(t, &trace.AccessDeniedError{}, trace.Unwrap(err))

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
			Expires:   time.Now().Add(time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.Error(t, err)
		require.IsType(t, &trace.AccessDeniedError{}, trace.Unwrap(err))
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
		require.Equal(t, identity.Expires.Sub(clock.Now()), time.Hour)
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
			Expires:        time.Now().Add(100 * time.Hour).UTC(),
			Format:         constants.CertificateFormatStandard,
			RouteToCluster: rc1.GetName(),
		})
		require.NoError(t, err)

		_, diff := parseCert(userCerts.SSH)
		require.Less(t, int64(diff), int64(testUser2.TTL))

		tlsCert, err := tlsca.ParseCertificatePEM(userCerts.TLS)
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)
		require.True(t, identity.Expires.Before(time.Now().Add(testUser2.TTL)))
		require.Equal(t, identity.RouteToCluster, rc1.GetName())
	})

	t.Run("Admin", func(t *testing.T) {
		// Admin should be allowed to generate certs with TTL longer than max.
		adminClient, err := srv.NewClient(TestAdmin())
		require.NoError(t, err)

		userCerts, err := adminClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
			PublicKey: pub,
			Username:  user1.GetName(),
			Expires:   time.Now().Add(40 * time.Hour).UTC(),
			Format:    constants.CertificateFormatStandard,
		})
		require.NoError(t, err)

		parsedCert, diff := parseCert(userCerts.SSH)
		require.Less(t, int64(apidefaults.MaxCertDuration), int64(diff))

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
			Expires:   time.Now().Add(1 * time.Hour).UTC(),
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
			Expires:   time.Now().Add(time.Hour).UTC(),
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
			Expires:        time.Now().Add(100 * time.Hour).UTC(),
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
			Expires:        time.Now().Add(100 * time.Hour).UTC(),
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
			Expires:        time.Now().Add(100 * time.Hour).UTC(),
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
func (s *TLSSuite) TestGenerateAppToken(c *check.C) {
	authClient, err := s.server.NewClient(TestBuiltin(types.RoleAdmin))
	c.Assert(err, check.IsNil)

	ca, err := authClient.GetCertAuthority(types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: s.server.ClusterName(),
	}, true)
	c.Assert(err, check.IsNil)

	signer, err := s.server.AuthServer.AuthServer.GetKeyStore().GetJWTSigner(ca)
	c.Assert(err, check.IsNil)
	key, err := services.GetJWTSigner(signer, ca.GetClusterName(), s.clock)
	c.Assert(err, check.IsNil)

	tests := []struct {
		inMachineRole types.SystemRole
		inComment     check.CommentInterface
		outError      bool
	}{
		{
			inMachineRole: types.RoleNode,
			inComment:     check.Commentf("nodes should not have the ability to generate tokens"),
			outError:      true,
		},
		{
			inMachineRole: types.RoleProxy,
			inComment:     check.Commentf("proxies should not have the ability to generate tokens"),
			outError:      true,
		},
		{
			inMachineRole: types.RoleApp,
			inComment:     check.Commentf("only apps should have the ability to generate tokens"),
			outError:      false,
		},
	}
	for _, tt := range tests {
		client, err := s.server.NewClient(TestBuiltin(tt.inMachineRole))
		c.Assert(err, check.IsNil, tt.inComment)

		token, err := client.GenerateAppToken(
			context.Background(),
			types.GenerateAppTokenRequest{
				Username: "foo@example.com",
				Roles:    []string{"bar", "baz"},
				URI:      "http://localhost:8080",
				Expires:  s.clock.Now().Add(1 * time.Minute),
			})
		c.Assert(err != nil, check.Equals, tt.outError, tt.inComment)
		if !tt.outError {
			claims, err := key.Verify(jwt.VerifyParams{
				Username: "foo@example.com",
				RawToken: token,
				URI:      "http://localhost:8080",
			})
			c.Assert(err, check.IsNil, tt.inComment)
			c.Assert(claims.Username, check.Equals, "foo@example.com", tt.inComment)
			c.Assert(claims.Roles, check.DeepEquals, []string{"bar", "baz"}, tt.inComment)
		}
	}
}

// TestCertificateFormat makes sure that certificates are generated with the
// correct format.
func (s *TLSSuite) TestCertificateFormat(c *check.C) {
	ctx := context.Background()
	priv, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	c.Assert(err, check.IsNil)
	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, check.IsNil)

	// use admin client to create user and role
	user, userRole, err := CreateUserAndRole(s.server.Auth(), "user", []string{"user"})
	c.Assert(err, check.IsNil)

	pass := []byte("very secure password")
	err = s.server.Auth().UpsertPassword(user.GetName(), pass)
	c.Assert(err, check.IsNil)

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

	for _, tt := range tests {
		roleOptions := userRole.GetOptions()
		roleOptions.CertificateFormat = tt.inRoleCertificateFormat
		userRole.SetOptions(roleOptions)
		err := s.server.Auth().UpsertRole(ctx, userRole)
		c.Assert(err, check.IsNil)

		proxyClient, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
		c.Assert(err, check.IsNil)

		// authentication attempt fails with password auth only
		re, err := proxyClient.AuthenticateSSHUser(AuthenticateSSHRequest{
			AuthenticateUserRequest: AuthenticateUserRequest{
				Username: user.GetName(),
				Pass: &PassCreds{
					Password: pass,
				},
			},
			CompatibilityMode: tt.inClientCertificateFormat,
			TTL:               apidefaults.CertDuration,
			PublicKey:         pub,
		})
		c.Assert(err, check.IsNil)

		parsedCert, err := sshutils.ParseCertificate(re.Cert)
		c.Assert(err, check.IsNil)

		_, ok := parsedCert.Extensions[teleport.CertExtensionTeleportRoles]
		c.Assert(ok, check.Equals, tt.outCertContainsRole)
	}
}

// TestClusterConfigContext checks that the cluster configuration gets passed
// along in the context and permissions get updated accordingly.
func (s *TLSSuite) TestClusterConfigContext(c *check.C) {
	ctx := context.Background()

	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	_, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// try and generate a host cert, this should fail because we are recording
	// at the nodes not at the proxy
	_, err = proxy.GenerateHostCert(pub,
		"a", "b", nil,
		"localhost", types.RoleProxy, 0)
	fixtures.ExpectAccessDenied(c, err)

	// update cluster config to record at the proxy
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetSessionRecordingConfig(ctx, recConfig)
	c.Assert(err, check.IsNil)

	// try and generate a host cert, now the proxy should be able to generate a
	// host cert because it's in recording mode.
	_, err = proxy.GenerateHostCert(pub,
		"a", "b", nil,
		"localhost", types.RoleProxy, 0)
	c.Assert(err, check.IsNil)
}

// TestAuthenticateWebUserOTP tests web authentication flow for password + OTP
func (s *TLSSuite) TestAuthenticateWebUserOTP(c *check.C) {
	ctx := context.Background()
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "ws-test"
	pass := []byte("ws-abc123")
	rawSecret := "def456"
	otpSecret := base32.StdEncoding.EncodeToString([]byte(rawSecret))

	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)

	err = s.server.Auth().UpsertPassword(user, pass)
	c.Assert(err, check.IsNil)

	dev, err := services.NewTOTPDevice("otp", otpSecret, s.clock.Now())
	c.Assert(err, check.IsNil)
	err = s.server.Auth().UpsertMFADevice(ctx, user, dev)
	c.Assert(err, check.IsNil)

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, s.clock.Now())
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetAuthPreference(ctx, authPreference)
	c.Assert(err, check.IsNil)

	// authentication attempt fails with wrong password
	_, err = proxy.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		OTP:      &OTPCreds{Password: []byte("wrong123"), Token: validToken},
	})
	fixtures.ExpectAccessDenied(c, err)

	// authentication attempt fails with wrong otp
	_, err = proxy.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		OTP:      &OTPCreds{Password: pass, Token: "wrong123"},
	})
	fixtures.ExpectAccessDenied(c, err)

	// authentication attempt fails with password auth only
	_, err = proxy.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass: &PassCreds{
			Password: pass,
		},
	})
	fixtures.ExpectAccessDenied(c, err)

	// authentication succeeds
	ws, err := proxy.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		OTP:      &OTPCreds{Password: pass, Token: validToken},
	})
	c.Assert(err, check.IsNil)

	userClient, err := s.server.NewClientFromWebSession(ws)
	c.Assert(err, check.IsNil)

	_, err = userClient.GetWebSessionInfo(context.TODO(), user, ws.GetName())
	c.Assert(err, check.IsNil)

	err = clt.DeleteWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)

	_, err = userClient.GetWebSessionInfo(context.TODO(), user, ws.GetName())
	c.Assert(err, check.NotNil)
}

// TestLoginAttempts makes sure the login attempt counter is incremented and
// reset correctly.
func (s *TLSSuite) TestLoginAttempts(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "user1"
	pass := []byte("abc123")

	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	err = clt.UpsertPassword(user, pass)
	c.Assert(err, check.IsNil)

	req := AuthenticateUserRequest{
		Username: user,
		Pass: &PassCreds{
			Password: []byte("bad pass"),
		},
	}
	// authentication attempt fails with bad password
	_, err = proxy.AuthenticateWebUser(req)
	fixtures.ExpectAccessDenied(c, err)

	// creates first failed login attempt
	loginAttempts, err := s.server.Auth().GetUserLoginAttempts(user)
	c.Assert(err, check.IsNil)
	c.Assert(loginAttempts, check.HasLen, 1)

	// try second time with wrong pass
	req.Pass.Password = pass
	_, err = proxy.AuthenticateWebUser(req)
	c.Assert(err, check.IsNil)

	// clears all failed attempts after success
	loginAttempts, err = s.server.Auth().GetUserLoginAttempts(user)
	c.Assert(err, check.IsNil)
	c.Assert(loginAttempts, check.HasLen, 0)
}

func (s *TLSSuite) TestChangeUserAuthentication(c *check.C) {
	ctx := context.Background()
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		AllowLocalAuth: types.NewBoolOption(true),
	})
	c.Assert(err, check.IsNil)

	err = s.server.Auth().SetAuthPreference(ctx, authPref)
	c.Assert(err, check.IsNil)

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	c.Assert(err, check.IsNil)

	err = s.server.Auth().SetAuthPreference(ctx, authPreference)
	c.Assert(err, check.IsNil)

	username := "user1"
	// Create a local user.
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	_, _, err = CreateUserAndRole(clt, username, []string{"role1"})
	c.Assert(err, check.IsNil)

	token, err := s.server.Auth().CreateResetPasswordToken(ctx, CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
	})
	c.Assert(err, check.IsNil)

	res, err := s.server.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:    token.GetName(),
		DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
	})
	c.Assert(err, check.IsNil)

	otpToken, err := totp.GenerateCode(res.GetTOTP().GetSecret(), s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	_, err = s.server.Auth().ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID:     token.GetName(),
		NewPassword: []byte("qweqweqwe"),
		NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{Code: otpToken},
		}},
	})
	c.Assert(err, check.IsNil)
}

// TestLoginNoLocalAuth makes sure that logins for local accounts can not be
// performed when local auth is disabled.
func (s *TLSSuite) TestLoginNoLocalAuth(c *check.C) {
	ctx := context.Background()
	user := "foo"
	pass := []byte("barbaz")

	// Create a local user.
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)
	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)
	err = clt.UpsertPassword(user, pass)
	c.Assert(err, check.IsNil)

	// Set auth preference to disallow local auth.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		AllowLocalAuth: types.NewBoolOption(false),
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetAuthPreference(ctx, authPref)
	c.Assert(err, check.IsNil)

	// Make sure access is denied for web login.
	_, err = s.server.Auth().AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass: &PassCreds{
			Password: pass,
		},
	})
	fixtures.ExpectAccessDenied(c, err)

	// Make sure access is denied for SSH login.
	_, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	_, err = s.server.Auth().AuthenticateSSHUser(AuthenticateSSHRequest{
		AuthenticateUserRequest: AuthenticateUserRequest{
			Username: user,
			Pass: &PassCreds{
				Password: pass,
			},
		},
		PublicKey: pub,
	})
	fixtures.ExpectAccessDenied(c, err)
}

// TestCipherSuites makes sure that clients with invalid cipher suites can
// not connect.
func (s *TLSSuite) TestCipherSuites(c *check.C) {
	otherServer, err := s.server.AuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer otherServer.Close()

	// Create a client with ciphersuites that the server does not support.
	tlsConfig, err := s.server.ClientTLSConfig(TestNop())
	c.Assert(err, check.IsNil)
	tlsConfig.CipherSuites = []uint16{
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	}

	addrs := []string{
		otherServer.Addr().String(),
		s.server.Addr().String(),
	}
	client, err := NewClient(client.Config{
		Addrs: addrs,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
	c.Assert(err, check.IsNil)

	// Requests should fail.
	_, err = client.GetClusterName()
	c.Assert(err, check.NotNil)
}

// TestTLSFailover tests client failover between two tls servers
func (s *TLSSuite) TestTLSFailover(c *check.C) {
	otherServer, err := s.server.AuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer otherServer.Close()

	tlsConfig, err := s.server.ClientTLSConfig(TestNop())
	c.Assert(err, check.IsNil)

	addrs := []string{
		otherServer.Addr().String(),
		s.server.Addr().String(),
	}
	client, err := NewClient(client.Config{
		Addrs: addrs,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
	c.Assert(err, check.IsNil)

	// couple of runs to get enough connections
	for i := 0; i < 4; i++ {
		_, err = client.GetDomainName()
		c.Assert(err, check.IsNil)
	}

	// stop the server to get response
	err = otherServer.Stop()
	c.Assert(err, check.IsNil)

	// client detects closed sockets and reconnecte to the backup server
	for i := 0; i < 4; i++ {
		_, err = client.GetDomainName()
		c.Assert(err, check.IsNil)
	}
}

// TestRegisterCAPin makes sure that registration only works with a valid
// CA pin.
func (s *TLSSuite) TestRegisterCAPin(c *check.C) {
	ctx := context.Background()
	// Generate a token to use.
	token, err := s.server.AuthServer.AuthServer.GenerateToken(ctx, GenerateTokenRequest{
		Roles: types.SystemRoles{
			types.RoleProxy,
		},
		TTL: time.Hour,
	})
	c.Assert(err, check.IsNil)

	// Generate public and private keys for node.
	priv, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	c.Assert(err, check.IsNil)
	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	c.Assert(err, check.IsNil)

	// Calculate what CA pin should be.
	localCAResponse, err := s.server.AuthServer.AuthServer.GetClusterCACert()
	c.Assert(err, check.IsNil)
	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	c.Assert(err, check.IsNil)
	c.Assert(caPins, check.HasLen, 1)
	caPin := caPins[0]

	// Attempt to register with valid CA pin, should work.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               []string{caPin},
		Clock:                s.clock,
	})
	c.Assert(err, check.IsNil)

	// Attempt to register with multiple CA pins where the auth server only
	// matches one, should work.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               []string{"sha256:123", caPin},
		Clock:                s.clock,
	})
	c.Assert(err, check.IsNil)

	// Attempt to register with invalid CA pin, should fail.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               []string{"sha256:123"},
		Clock:                s.clock,
	})
	c.Assert(err, check.NotNil)

	// Attempt to register with multiple invalid CA pins, should fail.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               []string{"sha256:123", "sha256:456"},
		Clock:                s.clock,
	})
	c.Assert(err, check.NotNil)

	// Add another cert to the CA (dupe the current one for simplicity)
	hostCA, err := s.server.AuthServer.AuthServer.GetCertAuthority(types.CertAuthID{
		DomainName: s.server.AuthServer.ClusterName,
		Type:       types.HostCA,
	}, true)
	c.Assert(err, check.IsNil)
	activeKeys := hostCA.GetActiveKeys()
	activeKeys.TLS = append(activeKeys.TLS, activeKeys.TLS...)
	hostCA.SetActiveKeys(activeKeys)
	err = s.server.AuthServer.AuthServer.UpsertCertAuthority(hostCA)
	c.Assert(err, check.IsNil)

	// Calculate what CA pins should be.
	localCAResponse, err = s.server.AuthServer.AuthServer.GetClusterCACert()
	c.Assert(err, check.IsNil)
	caPins, err = tlsca.CalculatePins(localCAResponse.TLSCA)
	c.Assert(err, check.IsNil)
	c.Assert(caPins, check.HasLen, 2)

	// Attempt to register with multiple CA pins, should work
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPins:               caPins,
		Clock:                s.clock,
	})
	c.Assert(err, check.IsNil)
}

// TestRegisterCAPath makes sure registration only works with a valid CA
// file on disk.
func (s *TLSSuite) TestRegisterCAPath(c *check.C) {
	ctx := context.Background()
	// Generate a token to use.
	token, err := s.server.AuthServer.AuthServer.GenerateToken(ctx, GenerateTokenRequest{
		Roles: types.SystemRoles{
			types.RoleProxy,
		},
		TTL: time.Hour,
	})
	c.Assert(err, check.IsNil)

	// Generate public and private keys for node.
	priv, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	c.Assert(err, check.IsNil)
	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	c.Assert(err, check.IsNil)

	// Attempt to register with nothing at the CA path, should work.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		Clock:                s.clock,
	})
	c.Assert(err, check.IsNil)

	// Extract the root CA public key and write it out to the data dir.
	hostCA, err := s.server.AuthServer.AuthServer.GetCertAuthority(types.CertAuthID{
		DomainName: s.server.AuthServer.ClusterName,
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	certs := services.GetTLSCerts(hostCA)
	c.Assert(certs, check.HasLen, 1)
	certPem := certs[0]
	caPath := filepath.Join(s.dataDir, defaults.CACertFile)
	err = ioutil.WriteFile(caPath, certPem, teleport.FileMaskOwnerOnly)
	c.Assert(err, check.IsNil)

	// Attempt to register with valid CA path, should work.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     types.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPath:               caPath,
		Clock:                s.clock,
	})
	c.Assert(err, check.IsNil)
}

// TestEventsNodePresence tests streaming node presence API -
// announcing node and keeping node alive
func (s *TLSSuite) TestEventsNodePresence(c *check.C) {
	ctx := context.Background()
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
	clt, err := s.server.NewClient(TestIdentity{
		I: BuiltinRole{
			Role:     types.RoleNode,
			Username: fmt.Sprintf("%v.%v", node.Metadata.Name, s.server.ClusterName()),
		},
	})
	c.Assert(err, check.IsNil)
	defer clt.Close()

	keepAlive, err := clt.UpsertNode(ctx, node)
	c.Assert(err, check.IsNil)
	c.Assert(keepAlive, check.NotNil)

	keepAliver, err := clt.NewKeepAliver(ctx)
	c.Assert(err, check.IsNil)
	defer keepAliver.Close()

	keepAlive.Expires = time.Now().Add(2 * time.Second)
	select {
	case keepAliver.KeepAlives() <- *keepAlive:
		// ok
	case <-time.After(time.Second):
		c.Fatalf("time out sending keep ailve")
	case <-keepAliver.Done():
		c.Fatalf("unknown problem sending keep ailve")
	}

	// upsert node and keep alives will fail for users with no privileges
	nopClt, err := s.server.NewClient(TestBuiltin(types.RoleNop))
	c.Assert(err, check.IsNil)
	defer nopClt.Close()

	_, err = nopClt.UpsertNode(ctx, node)
	fixtures.ExpectAccessDenied(c, err)

	k2, err := nopClt.NewKeepAliver(ctx)
	c.Assert(err, check.IsNil)

	keepAlive.Expires = time.Now().Add(2 * time.Second)
	go func() {
		select {
		case k2.KeepAlives() <- *keepAlive:
		case <-k2.Done():
		}
	}()

	select {
	case <-time.After(time.Second):
		c.Fatalf("time out expecting error")
	case <-k2.Done():
	}

	fixtures.ExpectAccessDenied(c, k2.Error())
}

// TestEventsPermissions tests events with regards
// to certificate authority rotation
func (s *TLSSuite) TestEventsPermissions(c *check.C) {
	clt, err := s.server.NewClient(TestBuiltin(types.RoleNode))
	c.Assert(err, check.IsNil)
	defer clt.Close()

	ctx := context.TODO()
	w, err := clt.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{{Kind: types.KindCertAuthority}}})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for init event")
	case event := <-w.Events():
		c.Assert(event.Type, check.Equals, types.OpInit)
	}

	// start rotation
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	suite.ExpectResource(c, w, 3*time.Second, ca)

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
		client, err := s.server.NewClient(tc.identity)
		c.Assert(err, check.IsNil)
		defer client.Close()

		watcher, err := client.NewWatcher(ctx, types.Watch{
			Kinds: tc.watches,
		})
		c.Assert(err, check.IsNil)
		defer watcher.Close()

		go func() {
			select {
			case <-watcher.Events():
			case <-watcher.Done():
			}
		}()

		select {
		case <-time.After(time.Second):
			c.Fatalf("time out expecting error in test %q", tc.name)
		case <-watcher.Done():
		}

		fixtures.ExpectAccessDenied(c, watcher.Error())
	}

	for _, tc := range testCases {
		tryWatch(tc)
	}
}

// TestEvents tests events suite
func (s *TLSSuite) TestEvents(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		ConfigS:       clt,
		EventsS:       clt,
		PresenceS:     clt,
		CAS:           clt,
		ProvisioningS: clt,
		Access:        clt,
		UsersS:        clt,
	}
	suite.Events(c)
}

// TestEventsClusterConfig test cluster configuration
func (s *TLSSuite) TestEventsClusterConfig(c *check.C) {
	clt, err := s.server.NewClient(TestBuiltin(types.RoleAdmin))
	c.Assert(err, check.IsNil)
	defer clt.Close()

	ctx := context.TODO()
	w, err := clt.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: true},
		{Kind: types.KindStaticTokens},
		{Kind: types.KindToken},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterName},
	}})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for init event")
	case event := <-w.Events():
		c.Assert(event.Type, check.Equals, types.OpInit)
	}

	// start rotation
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        types.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(types.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       types.HostCA,
	}, true)
	c.Assert(err, check.IsNil)

	suite.ExpectResource(c, w, 3*time.Second, ca)

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
	c.Assert(err, check.IsNil)

	err = s.server.Auth().SetStaticTokens(staticTokens)
	c.Assert(err, check.IsNil)

	staticTokens, err = s.server.Auth().GetStaticTokens()
	c.Assert(err, check.IsNil)
	suite.ExpectResource(c, w, 3*time.Second, staticTokens)

	// create provision token and expect the update event
	token, err := types.NewProvisionToken(
		"tok2", types.SystemRoles{types.RoleProxy}, time.Now().UTC().Add(3*time.Hour))
	c.Assert(err, check.IsNil)

	err = s.server.Auth().UpsertToken(ctx, token)
	c.Assert(err, check.IsNil)

	token, err = s.server.Auth().GetToken(ctx, token.GetName())
	c.Assert(err, check.IsNil)

	suite.ExpectResource(c, w, 3*time.Second, token)

	// delete token and expect delete event
	err = s.server.Auth().DeleteToken(ctx, token.GetName())
	c.Assert(err, check.IsNil)
	suite.ExpectDeleteResource(c, w, 3*time.Second, &types.ResourceHeader{
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
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetClusterAuditConfig(ctx, auditConfig)
	c.Assert(err, check.IsNil)

	auditConfigResource, err := s.server.Auth().GetClusterAuditConfig(ctx)
	c.Assert(err, check.IsNil)
	suite.ExpectResource(c, w, 3*time.Second, auditConfigResource)

	// update cluster name resource metadata
	clusterNameResource, err := s.server.Auth().GetClusterName()
	c.Assert(err, check.IsNil)

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

	err = s.server.Auth().DeleteClusterName()
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetClusterName(clusterName)
	c.Assert(err, check.IsNil)

	clusterNameResource, err = s.server.Auth().ClusterConfiguration.GetClusterName()
	c.Assert(err, check.IsNil)
	suite.ExpectResource(c, w, 3*time.Second, clusterNameResource)
}

func (s *TLSSuite) TestNetworkRestrictions(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		RestrictionsS: clt,
	}
	suite.NetworkRestrictions(c)
}

// verifyJWT verifies that the token was signed by one the passed in key pair.
func (s *TLSSuite) verifyJWT(clock clockwork.Clock, clusterName string, pairs []*types.JWTKeyPair, token string) (*jwt.Claims, error) {
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
			URI:      "http://localhost:8080",
		})
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		return claims, nil
	}
	return nil, trace.NewAggregate(errs...)
}

func newTestTLSServer(t *testing.T) *TestTLSServer {
	as, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, srv.Close()) })
	return srv
}
