/*
Copyright 2017 Gravitational, Inc.

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
	"crypto/tls"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"gopkg.in/check.v1"
)

type TLSSuite struct {
	dataDir string
	server  *TestTLSServer
}

var _ = check.Suite(&TLSSuite{})

func (s *TLSSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *TLSSuite) SetUpTest(c *check.C) {
	s.dataDir = c.MkDir()

	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir: s.dataDir,
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
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
	})
	c.Assert(err, check.IsNil)

	certPool, err := s.server.CertPool()
	c.Assert(err, check.IsNil)

	// without trust, proxy server will get rejected
	// remote auth server will get rejected because it is not supported
	remoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	// certificate authority is not recognized, because
	// the trust has not been established yet
	_, err = remoteProxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// after trust is established, things are good
	err = s.server.AuthServer.Trust(remoteServer, nil)

	_, err = remoteProxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// remote auth server will get rejected even with established trust
	remoteAuth, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleAuth), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	_, err = remoteAuth.GetDomainName()
	fixtures.ExpectAccessDenied(c, err)
}

// TestAcceptedUsage tests scenario when server is set up
// to accept certificates with certain usage metadata restrictions
// encoded
func (s *TLSSuite) TestAcceptedUsage(c *check.C) {
	server, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:           c.MkDir(),
		ClusterName:   "remote",
		AcceptedUsage: []string{"usage:k8s"},
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
	_, err = client.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// restricted clients can use restricted servers if restrictions
	// exactly match
	identity := TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:k8s"}
	client, err = tlsServer.NewClient(identity)
	c.Assert(err, check.IsNil)

	_, err = client.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// restricted clients can will be rejected if usage does not match
	identity = TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:extra"}
	client, err = tlsServer.NewClient(identity)
	c.Assert(err, check.IsNil)

	_, err = client.GetNodes(defaults.Namespace, services.SkipValidation())
	fixtures.ExpectAccessDenied(c, err)

	// restricted clients can will be rejected, for now if there is any mismatch,
	// including extra usage.
	identity = TestUser(user.GetName())
	identity.AcceptedUsage = []string{"usage:k8s", "usage:unknown"}
	client, err = tlsServer.NewClient(identity)
	c.Assert(err, check.IsNil)

	_, err = client.GetNodes(defaults.Namespace, services.SkipValidation())
	fixtures.ExpectAccessDenied(c, err)

}

// TestRemoteRotation tests remote builtin role
// that attempts certificate authority rotation
func (s *TLSSuite) TestRemoteRotation(c *check.C) {
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
	})
	c.Assert(err, check.IsNil)

	certPool, err := s.server.CertPool()
	c.Assert(err, check.IsNil)

	// after trust is established, things are good
	err = s.server.AuthServer.Trust(remoteServer, nil)

	remoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	remoteAuth, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleAuth), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	// remote cluster starts rotation
	gracePeriod := time.Hour
	remoteServer.AuthServer.privateKey = fixtures.PEMBytes["rsa2"]
	err = remoteServer.AuthServer.RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// moves to update clients
	err = remoteServer.AuthServer.RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseUpdateClients,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	remoteCA, err := remoteServer.AuthServer.GetCertAuthority(services.CertAuthID{
		DomainName: remoteServer.ClusterName,
		Type:       services.HostCA,
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

	// remote auth server will get rejected
	err = remoteAuth.RotateExternalCertAuthority(remoteCA)
	fixtures.ExpectAccessDenied(c, err)

	// remote proxy should be able to perform remote cert authority
	// rotation
	err = remoteProxy.RotateExternalCertAuthority(remoteCA)
	c.Assert(err, check.IsNil)

	// newRemoteProxy should be trusted by the auth server
	newRemoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	_, err = newRemoteProxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// old proxy client is still trusted
	_, err = s.server.CloneClient(remoteProxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)
}

// TestLocalProxyPermissions tests new local proxy permissions
// as it's now allowed to update host cert authorities of remote clusters
func (s *TLSSuite) TestLocalProxyPermissions(c *check.C) {
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
	})
	c.Assert(err, check.IsNil)

	// after trust is established, things are good
	err = s.server.AuthServer.Trust(remoteServer, nil)
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// local proxy can't update local cert authorities
	err = proxy.UpsertCertAuthority(ca)
	fixtures.ExpectAccessDenied(c, err)

	// local proxy is allowed to update host CA of remote cert authorities
	remoteCA, err := s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: remoteServer.ClusterName,
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	err = proxy.UpsertCertAuthority(remoteCA)
	c.Assert(err, check.IsNil)
}

// TestAutoRotation tests local automatic rotation
func (s *TLSSuite) TestAutoRotation(c *check.C) {
	clock := clockwork.NewFakeClockAt(time.Now().Add(-2 * time.Hour))
	s.server.Auth().SetClock(clock)

	// create proxy client
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// starts rotation
	s.server.Auth().privateKey = fixtures.PEMBytes["rsa2"]
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		Mode:        services.RotationModeAuto,
	})
	c.Assert(err, check.IsNil)

	// advance rotation by clock
	clock.Advance(gracePeriod/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, services.RotationPhaseUpdateClients)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// new clients work as well
	newProxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// advance rotation by clock
	clock.Advance((gracePeriod*2)/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	c.Assert(err, check.IsNil)

	ca, err = s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, services.RotationPhaseUpdateServers)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// new clients work as well
	newProxy, err = s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	_, err = newProxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// complete rotation - advance rotation by clock
	clock.Advance(gracePeriod/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	ca, err = s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, services.RotationPhaseStandby)
	c.Assert(err, check.IsNil)

	// old clients should no longer work
	// new client has to be created here to force re-create the new
	// connection instead of re-using the one from pool
	// this is not going to be a problem in real teleport
	// as it reloads the full server after reload
	_, err = s.server.CloneClient(proxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// new clients work
	_, err = s.server.CloneClient(newProxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)
}

// TestAutoFallback tests local automatic rotation fallback,
// when user intervenes with rollback and rotation gets switched
// to manual mode
func (s *TLSSuite) TestAutoFallback(c *check.C) {
	clock := clockwork.NewFakeClockAt(time.Now().Add(-2 * time.Hour))
	s.server.Auth().SetClock(clock)

	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// starts rotation
	s.server.Auth().privateKey = fixtures.PEMBytes["rsa2"]
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		Mode:        services.RotationModeAuto,
	})
	c.Assert(err, check.IsNil)

	// advance rotation by clock
	clock.Advance(gracePeriod/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, services.RotationPhaseUpdateClients)
	c.Assert(ca.GetRotation().Mode, check.Equals, services.RotationModeAuto)

	// rollback rotation
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseRollback,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	ca, err = s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	c.Assert(ca.GetRotation().Phase, check.Equals, services.RotationPhaseRollback)
	c.Assert(ca.GetRotation().Mode, check.Equals, services.RotationModeManual)
}

// TestManualRotation tests local manual rotation
// that performs full-cycle certificate authority rotation
func (s *TLSSuite) TestManualRotation(c *check.C) {
	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// can't jump to mid-phase
	gracePeriod := time.Hour
	s.server.Auth().privateKey = fixtures.PEMBytes["rsa2"]
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseUpdateServers,
		Mode:        services.RotationModeManual,
	})
	fixtures.ExpectBadParameter(c, err)

	// starts rotation
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// clients reconnect
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseUpdateClients,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// new clients work as well
	newProxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	_, err = newProxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// can't jump to standy
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseStandby,
		Mode:        services.RotationModeManual,
	})
	fixtures.ExpectBadParameter(c, err)

	// advance rotation:
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseUpdateServers,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// old clients should work
	_, err = s.server.CloneClient(proxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// new clients work as well
	_, err = s.server.CloneClient(newProxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// complete rotation
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseStandby,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// old clients should no longer work
	// new client has to be created here to force re-create the new
	// connection instead of re-using the one from pool
	// this is not going to be a problem in real teleport
	// as it reloads the full server after reload
	_, err = s.server.CloneClient(proxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// new clients work
	_, err = s.server.CloneClient(newProxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)
}

// TestRollback tests local manual rotation rollback
func (s *TLSSuite) TestRollback(c *check.C) {
	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// starts rotation
	gracePeriod := time.Hour
	s.server.Auth().privateKey = fixtures.PEMBytes["rsa2"]

	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// move to update clients phase
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseUpdateClients,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// new clients work
	newProxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	_, err = newProxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// advance rotation:
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseUpdateServers,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// rollback rotation
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseRollback,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// new clients work, server still accepts the creds
	// because new clients should re-register and receive new certs
	_, err = s.server.CloneClient(newProxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// can't jump to other phases
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseUpdateClients,
		Mode:        services.RotationModeManual,
	})
	fixtures.ExpectBadParameter(c, err)

	// complete rollback
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseStandby,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// clients with new creds will no longer work
	_, err = s.server.CloneClient(newProxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.ErrorMatches, ".*bad certificate.*")

	// clients with old creds will still work
	_, err = s.server.CloneClient(proxy).GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)
}

// TestRemoteUser tests scenario when remote user connects to the local
// auth server and some edge cases.
func (s *TLSSuite) TestRemoteUser(c *check.C) {
	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
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

	err = s.server.AuthServer.Trust(remoteServer, services.RoleMap{{Remote: remoteRole.GetName(), Local: []string{localRole.GetName()}}})
	c.Assert(err, check.IsNil)

	_, err = remoteClient.GetDomainName()
	c.Assert(err, check.IsNil)
}

// TestNopUser tests user with no permissions except
// the ones that require other authentication methods ("nop" user)
func (s *TLSSuite) TestNopUser(c *check.C) {
	client, err := s.server.NewClient(TestNop())
	c.Assert(err, check.IsNil)

	// Nop User can get cluster name
	_, err = client.GetDomainName()
	c.Assert(err, check.IsNil)

	// But can not get users or nodes
	_, err = client.GetUsers()
	fixtures.ExpectAccessDenied(c, err)

	_, err = client.GetNodes(defaults.Namespace, services.SkipValidation())
	fixtures.ExpectAccessDenied(c, err)
}

// TestOwnRole tests that user can read roles assigned to them
func (s *TLSSuite) TestReadOwnRole(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user1, userRole, err := CreateUserAndRoleWithoutRoles(clt, "user1", []string{"user1"})
	c.Assert(err, check.IsNil)

	user2, _, err := CreateUserAndRoleWithoutRoles(clt, "user2", []string{"user2"})
	c.Assert(err, check.IsNil)

	// user should be able to read their own roles
	userClient, err := s.server.NewClient(TestUser(user1.GetName()))
	c.Assert(err, check.IsNil)

	_, err = userClient.GetRole(userRole.GetName())
	c.Assert(err, check.IsNil)

	// user2 can't read user1 role
	userClient2, err := s.server.NewClient(TestIdentity{I: LocalUser{Username: user2.GetName()}})
	c.Assert(err, check.IsNil)

	_, err = userClient2.GetRole(userRole.GetName())
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

func (s *TLSSuite) TestClusterConfig(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		ConfigS: clt,
	}
	suite.ClusterConfig(c)
}

func (s *TLSSuite) TestTunnelConnectionsCRUD(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		PresenceS: clt,
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

	users, err := clt.GetUsers()
	c.Assert(err, check.IsNil)
	c.Assert(len(users), check.Equals, 1)
	c.Assert(users[0].GetName(), check.Equals, "user1")

	c.Assert(clt.DeleteUser("user1"), check.IsNil)

	users, err = clt.GetUsers()
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

	err = s.server.Auth().UpsertTOTP("user1", otpSecret)
	c.Assert(err, check.IsNil)

	validToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	err = clt.CheckPassword("user1", pass, validToken)
	c.Assert(err, check.IsNil)
}

func (s *TLSSuite) TestTokens(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	out, err := clt.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleNode}})
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Not(check.Equals), 0)
}

func (s *TLSSuite) TestSharedSessions(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	out, err := clt.GetSessions(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.DeepEquals, []session.Session{})

	date := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	sess := session.Session{
		Active:         true,
		ID:             session.NewID(),
		TerminalParams: session.TerminalParams{W: 100, H: 100},
		Created:        date,
		LastActive:     date,
		Login:          "bob",
		Namespace:      defaults.Namespace,
	}
	c.Assert(clt.CreateSession(sess), check.IsNil)

	out, err = clt.GetSessions(defaults.Namespace)
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
	err = os.MkdirAll(filepath.Join(uploadDir, "upload", "sessions", defaults.Namespace), 0755)
	forwarder, err := events.NewForwarder(events.ForwarderConfig{
		Namespace:      defaults.Namespace,
		SessionID:      sess.ID,
		ServerID:       teleport.ComponentUpload,
		DataDir:        uploadDir,
		RecordSessions: true,
		ForwardTo:      clt,
	})
	c.Assert(err, check.IsNil)

	err = forwarder.PostSessionSlice(events.SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: string(sess.ID),
		Chunks: []*events.SessionChunk{
			&events.SessionChunk{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  events.SessionStartEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "bob", "val": "one"}),
			},
			&events.SessionChunk{
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
		Namespace:      defaults.Namespace,
		SessionID:      sess.ID,
		ServerID:       teleport.ComponentUpload,
		DataDir:        uploadDir,
		RecordSessions: true,
		ForwardTo:      clt,
	})
	c.Assert(err, check.IsNil)
	err = clt.PostSessionSlice(events.SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: string(anotherSessionID),
		Chunks: []*events.SessionChunk{
			&events.SessionChunk{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  events.SessionStartEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "alice", "val": "three"}),
			},
			&events.SessionChunk{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 1,
				EventType:  events.SessionEndEvent,
				Data:       marshal(events.EventFields{events.EventLogin: "alice", "val": "three"}),
			},
		},
		Version: events.V3,
	})
	c.Assert(err, check.IsNil)
	c.Assert(forwarder.Close(), check.IsNil)

	// start uploader process
	eventsC := make(chan *events.UploadEvent, 100)
	uploader, err := events.NewUploader(events.UploaderConfig{
		ServerID:   "upload",
		DataDir:    uploadDir,
		Namespace:  defaults.Namespace,
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
	e, err := clt.GetSessionEvents(defaults.Namespace, sess.ID, 0, true)
	c.Assert(err, check.IsNil)
	c.Assert(len(e), check.Equals, 2)
	c.Assert(e[0].GetString("val"), check.Equals, "one")
	c.Assert(e[1].GetString("val"), check.Equals, "two")

	// try searching for events with no filter (empty query) - should get all 3 events:
	to := time.Now().In(time.UTC).Add(time.Hour)
	from := to.Add(-time.Hour * 2)
	history, err := clt.SearchEvents(from, to, "", 0)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.NotNil)
	c.Assert(len(history), check.Equals, 4)

	// try searching for only "session.end" events (real query)
	history, err = clt.SearchEvents(from, to,
		fmt.Sprintf("%s=%s", events.EventType, events.SessionEndEvent), 0)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.NotNil)
	c.Assert(len(history), check.Equals, 2)
	var found bool
	for _, event := range history {
		if event.GetString(events.SessionEventID) == string(anotherSessionID) {
			found = true
			c.Assert(event.GetString("val"), check.Equals, "three")
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
	err = s.server.Auth().UpsertTOTP(user, otpSecret)
	c.Assert(err, check.IsNil)

	// make sure the otp url we get back is valid url issued to the correct user
	otpURL, _, err := s.server.Auth().GetOTPData(user)
	c.Assert(err, check.IsNil)
	u, err := url.Parse(otpURL)
	c.Assert(err, check.IsNil)
	c.Assert(u.Path, check.Equals, "/user1")

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
func (s *TLSSuite) TestWebSessions(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	user := "user1"
	pass := []byte("abc123")

	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
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

	// success with password set up
	ws, err := proxy.AuthenticateWebUser(req)
	c.Assert(err, check.IsNil)
	c.Assert(ws, check.Not(check.Equals), "")

	web, err := s.server.NewClientFromWebSession(ws)
	c.Assert(err, check.IsNil)

	_, err = web.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.IsNil)

	new, err := web.ExtendWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)
	c.Assert(new, check.NotNil)

	// Requesting forbidden action for user fails
	err = web.DeleteUser(user)
	fixtures.ExpectAccessDenied(c, err)

	err = clt.DeleteWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)

	_, err = web.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.NotNil)

	_, err = web.ExtendWebSession(user, ws.GetName())
	c.Assert(err, check.NotNil)
}

// TestGenerateCerts tests edge cases around authorization of
// certificate generation for servers and users
func (s *TLSSuite) TestGenerateCerts(c *check.C) {
	priv, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// make sure we can parse the private and public key
	privateKey, err := ssh.ParseRawPrivateKey(priv)
	c.Assert(err, check.IsNil)

	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	c.Assert(err, check.IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, check.IsNil)

	// generate server keys for node
	hostID := "00000000-0000-0000-0000-000000000000"
	hostClient, err := s.server.NewClient(TestIdentity{I: BuiltinRole{Username: hostID, Role: teleport.RoleNode}})
	c.Assert(err, check.IsNil)

	certs, err := hostClient.GenerateServerKeys(
		GenerateServerKeysRequest{
			HostID:               hostID,
			NodeName:             s.server.AuthServer.ClusterName,
			Roles:                teleport.Roles{teleport.RoleNode},
			AdditionalPrincipals: []string{"example.com"},
		})
	c.Assert(err, check.IsNil)

	key, _, _, _, err := ssh.ParseAuthorizedKey(certs.Cert)
	c.Assert(err, check.IsNil)
	hostCert := key.(*ssh.Certificate)
	comment := check.Commentf("can't find example.com in %v", hostCert.ValidPrincipals)
	c.Assert(utils.SliceContainsStr(hostCert.ValidPrincipals, "example.com"), check.Equals, true, comment)

	// sign server public keys for node
	hostID = "00000000-0000-0000-0000-000000000000"
	hostClient, err = s.server.NewClient(TestIdentity{I: BuiltinRole{Username: hostID, Role: teleport.RoleNode}})
	c.Assert(err, check.IsNil)

	certs, err = hostClient.GenerateServerKeys(
		GenerateServerKeysRequest{
			HostID:               hostID,
			NodeName:             s.server.AuthServer.ClusterName,
			Roles:                teleport.Roles{teleport.RoleNode},
			AdditionalPrincipals: []string{"example.com"},
			PublicSSHKey:         pub,
			PublicTLSKey:         pubTLS,
		})
	c.Assert(err, check.IsNil)

	key, _, _, _, err = ssh.ParseAuthorizedKey(certs.Cert)
	c.Assert(err, check.IsNil)
	hostCert = key.(*ssh.Certificate)
	comment = check.Commentf("can't find example.com in %v", hostCert.ValidPrincipals)
	c.Assert(utils.SliceContainsStr(hostCert.ValidPrincipals, "example.com"), check.Equals, true, comment)

	// attempt to elevate privileges by getting admin role in the certificate
	_, err = hostClient.GenerateServerKeys(
		GenerateServerKeysRequest{
			HostID:   hostID,
			NodeName: s.server.AuthServer.ClusterName,
			Roles:    teleport.Roles{teleport.RoleAdmin},
		})
	fixtures.ExpectAccessDenied(c, err)

	// attempt to get certificate for different host id
	_, err = hostClient.GenerateServerKeys(GenerateServerKeysRequest{
		HostID:   "some-other-host-id",
		NodeName: s.server.AuthServer.ClusterName,
		Roles:    teleport.Roles{teleport.RoleNode},
	})
	fixtures.ExpectAccessDenied(c, err)

	user1, userRole, err := CreateUserAndRole(s.server.Auth(), "user1", []string{"user1"})
	c.Assert(err, check.IsNil)

	user2, _, err := CreateUserAndRole(s.server.Auth(), "user2", []string{"user2"})
	c.Assert(err, check.IsNil)

	// unauthenticated client should NOT be able to generate a user cert without auth
	nopClient, err := s.server.NewClient(TestNop())
	c.Assert(err, check.IsNil)

	_, err = nopClient.GenerateUserCert(pub, user1.GetName(), time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.NotNil)
	fixtures.ExpectAccessDenied(c, err)
	c.Assert(err, check.ErrorMatches, "this request can be only executed by an admin")

	// Users don't match
	userClient2, err := s.server.NewClient(TestUser(user2.GetName()))
	c.Assert(err, check.IsNil)

	_, err = userClient2.GenerateUserCert(pub, user1.GetName(), time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.NotNil)
	fixtures.ExpectAccessDenied(c, err)
	c.Assert(err, check.ErrorMatches, "this request can be only executed by an admin")

	// Admin should be allowed to generate certs with TTL longer than max.
	adminClient, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	cert, err := adminClient.GenerateUserCert(pub, user1.GetName(), 40*time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.IsNil)

	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	c.Assert(err, check.IsNil)
	parsedCert, _ := parsedKey.(*ssh.Certificate)
	validBefore := time.Unix(int64(parsedCert.ValidBefore), 0)
	diff := validBefore.Sub(time.Now())
	c.Assert(diff > defaults.MaxCertDuration, check.Equals, true, check.Commentf("expected %v > %v", diff, defaults.CertDuration))

	// user should have agent forwarding (default setting)
	_, exists := parsedCert.Extensions[teleport.CertExtensionPermitAgentForwarding]
	c.Assert(exists, check.Equals, true)

	// now update role to permit agent forwarding
	roleOptions := userRole.GetOptions()
	roleOptions.ForwardAgent = services.NewBool(true)
	userRole.SetOptions(roleOptions)
	err = s.server.Auth().UpsertRole(userRole, backend.Forever)
	c.Assert(err, check.IsNil)

	cert, err = adminClient.GenerateUserCert(pub, user1.GetName(), 1*time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.IsNil)
	parsedKey, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, check.IsNil)
	parsedCert, _ = parsedKey.(*ssh.Certificate)

	// user should get agent forwarding
	_, exists = parsedCert.Extensions[teleport.CertExtensionPermitAgentForwarding]
	c.Assert(exists, check.Equals, true)

	// apply HTTP Auth to generate user cert:
	cert, err = adminClient.GenerateUserCert(pub, user1.GetName(), time.Hour, teleport.CertificateFormatStandard)
	c.Assert(err, check.IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, check.IsNil)
}

// TestCertificateFormat makes sure that certificates are generated with the
// correct format.
func (s *TLSSuite) TestCertificateFormat(c *check.C) {
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

	var tests = []struct {
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
			teleport.CertificateFormatStandard,
			true,
		},
	}

	for _, tt := range tests {
		roleOptions := userRole.GetOptions()
		roleOptions.CertificateFormat = tt.inRoleCertificateFormat
		userRole.SetOptions(roleOptions)
		err := s.server.Auth().UpsertRole(userRole, backend.Forever)
		c.Assert(err, check.IsNil)

		proxyClient, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
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
			TTL:               defaults.CertDuration,
			PublicKey:         pub,
		})
		c.Assert(err, check.IsNil)

		parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(re.Cert)
		c.Assert(err, check.IsNil)
		parsedCert, _ := parsedKey.(*ssh.Certificate)

		_, ok := parsedCert.Extensions[teleport.CertExtensionTeleportRoles]
		c.Assert(ok, check.Equals, tt.outCertContainsRole)
	}
}

// TestClusterConfigContext checks that the cluster configuration gets passed
// along in the context and permissions get updated accordingly.
func (s *TLSSuite) TestClusterConfigContext(c *check.C) {
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	_, pub, err := s.server.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// try and generate a host cert, this should fail because we are recording
	// at the nodes not at the proxy
	_, err = proxy.GenerateHostCert(pub,
		"a", "b", nil,
		"localhost", teleport.Roles{teleport.RoleProxy}, 0)
	fixtures.ExpectAccessDenied(c, err)

	// update cluster config to record at the proxy
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording: services.RecordAtProxy,
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	// try and generate a host cert, now the proxy should be able to generate a
	// host cert because it's in recording mode.
	_, err = proxy.GenerateHostCert(pub,
		"a", "b", nil,
		"localhost", teleport.Roles{teleport.RoleProxy}, 0)
	c.Assert(err, check.IsNil)
}

// TestAuthenticateWebUserOTP tests web authentication flow for password + OTP
func (s *TLSSuite) TestAuthenticateWebUserOTP(c *check.C) {
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

	err = s.server.Auth().UpsertTOTP(user, otpSecret)
	c.Assert(err, check.IsNil)

	otpURL, _, err := s.server.Auth().GetOTPData(user)
	c.Assert(err, check.IsNil)

	// make sure label in url is correct
	u, err := url.Parse(otpURL)
	c.Assert(err, check.IsNil)
	c.Assert(u.Path, check.Equals, "/ws-test")

	// create a valid otp token
	validToken, err := totp.GenerateCode(otpSecret, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetAuthPreference(authPreference)
	c.Assert(err, check.IsNil)

	// authentication attempt fails with wrong passwrod
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

	_, err = userClient.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.IsNil)

	err = clt.DeleteWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)

	_, err = userClient.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.NotNil)
}

// TestTokenSignupFlow tests signup flow using invite token
func (s *TLSSuite) TestTokenSignupFlow(c *check.C) {
	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetAuthPreference(authPreference)
	c.Assert(err, check.IsNil)

	user := "foobar"
	mappings := []string{"admin", "db"}

	token, err := s.server.Auth().CreateSignupToken(services.UserV1{Name: user, AllowedLogins: mappings}, 0)
	c.Assert(err, check.IsNil)

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// invalid token
	_, _, err = proxy.GetSignupTokenData("bad_token_data")
	c.Assert(err, check.NotNil)

	// valid token - success
	_, _, err = proxy.GetSignupTokenData(token)
	c.Assert(err, check.IsNil)

	signupToken, err := s.server.Auth().GetSignupToken(token)
	c.Assert(err, check.IsNil)

	otpToken, err := totp.GenerateCode(signupToken.OTPKey, s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	// valid token, but missing second factor
	newPassword := "abc123"
	_, err = proxy.CreateUserWithoutOTP(token, newPassword)
	fixtures.ExpectAccessDenied(c, err)

	// invalid signup token
	_, err = proxy.CreateUserWithOTP("what_token?", newPassword, otpToken)
	fixtures.ExpectAccessDenied(c, err)

	// valid signup token, invalid otp token
	_, err = proxy.CreateUserWithOTP(token, newPassword, "badotp")
	fixtures.ExpectAccessDenied(c, err)

	// success
	ws, err := proxy.CreateUserWithOTP(token, newPassword, otpToken)
	c.Assert(err, check.IsNil)

	// attempt to reuse token fails
	_, err = proxy.CreateUserWithOTP(token, newPassword, otpToken)
	fixtures.ExpectAccessDenied(c, err)

	// can login with web session credentials now
	userClient, err := s.server.NewClientFromWebSession(ws)
	c.Assert(err, check.IsNil)

	_, err = userClient.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.IsNil)
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

	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
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

	addrs := []utils.NetAddr{
		utils.FromAddr(otherServer.Listener.Addr()),
		utils.FromAddr(s.server.Listener.Addr()),
	}
	client, err := NewTLSClient(addrs, tlsConfig)
	c.Assert(err, check.IsNil)

	// Requests should fail.
	_, err = client.GetDomainName()
	c.Assert(err, check.NotNil)
}

// TestTLSFailover tests client failover between two tls servers
func (s *TLSSuite) TestTLSFailover(c *check.C) {
	otherServer, err := s.server.AuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer otherServer.Close()

	tlsConfig, err := s.server.ClientTLSConfig(TestNop())
	c.Assert(err, check.IsNil)

	addrs := []utils.NetAddr{
		utils.FromAddr(otherServer.Listener.Addr()),
		utils.FromAddr(s.server.Listener.Addr()),
	}
	client, err := NewTLSClient(addrs, tlsConfig)
	c.Assert(err, check.IsNil)

	// couple of runs to get enough connections
	for i := 0; i < 4; i++ {
		_, err = client.GetDomainName()
		c.Assert(err, check.IsNil)
	}

	// stop the server to get response
	otherServer.Stop()

	// client detects closed sockets and reconnecte to the backup server
	for i := 0; i < 4; i++ {
		_, err = client.GetDomainName()
		c.Assert(err, check.IsNil)
	}
}

// TestRegisterCAPin makes sure that registration only works with a valid
// CA pin.
func (s *TLSSuite) TestRegisterCAPin(c *check.C) {
	// Generate a token to use.
	token, err := s.server.AuthServer.AuthServer.GenerateToken(GenerateTokenRequest{
		Roles: teleport.Roles{
			teleport.RoleProxy,
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
	hostCA, err := s.server.AuthServer.AuthServer.GetCertAuthority(services.CertAuthID{
		DomainName: s.server.AuthServer.ClusterName,
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	tlsCA, err := hostCA.TLSCA()
	c.Assert(err, check.IsNil)
	caPin := utils.CalculateSKPI(tlsCA.Cert)

	// Attempt to register with valid CA pin, should work.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     teleport.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPin:                caPin,
	})
	c.Assert(err, check.IsNil)

	// Attempt to register with invalid CA pin, should fail.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     teleport.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPin:                "sha256:123",
	})
	c.Assert(err, check.NotNil)
}

// TestRegisterCAPath makes sure registration only works with a valid CA
// file on disk.
func (s *TLSSuite) TestRegisterCAPath(c *check.C) {
	// Generate a token to use.
	token, err := s.server.AuthServer.AuthServer.GenerateToken(GenerateTokenRequest{
		Roles: teleport.Roles{
			teleport.RoleProxy,
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
			Role:     teleport.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
	})
	c.Assert(err, check.IsNil)

	// Extract the root CA public key and write it out to the data dir.
	hostCA, err := s.server.AuthServer.AuthServer.GetCertAuthority(services.CertAuthID{
		DomainName: s.server.AuthServer.ClusterName,
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	tlsCA, err := hostCA.TLSCA()
	c.Assert(err, check.IsNil)
	tlsBytes, err := tlsca.MarshalCertificatePEM(tlsCA.Cert)
	c.Assert(err, check.IsNil)
	caPath := filepath.Join(s.dataDir, defaults.CACertFile)
	err = ioutil.WriteFile(caPath, tlsBytes, teleport.FileMaskOwnerOnly)
	c.Assert(err, check.IsNil)

	// Attempt to register with valid CA path, should work.
	_, err = Register(RegisterParams{
		Servers: []utils.NetAddr{utils.FromAddr(s.server.Addr())},
		Token:   token,
		ID: IdentityID{
			HostUUID: "once",
			NodeName: "node-name",
			Role:     teleport.RoleProxy,
		},
		AdditionalPrincipals: []string{"example.com"},
		PrivateKey:           priv,
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
		CAPath:               caPath,
	})
	c.Assert(err, check.IsNil)
}
