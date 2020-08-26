/*
Copyright 2017-2019 Gravitational, Inc.

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
	"google.golang.org/grpc"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace/trail"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"gopkg.in/check.v1"
)

// MockAuthServiceClient stubs grpc AuthServiceClient interface.
type mockAuthServiceClient struct {
	proto.AuthServiceClient
	err error
}

// newMockAuthServiceClient returns a new instace of mockAuthServiceClient
func newMockAuthServiceClient() *mockAuthServiceClient {
	return &mockAuthServiceClient{}
}

// DeleteUser returns a dynamically defined method.
func (m *mockAuthServiceClient) DeleteUser(ctx context.Context, in *proto.DeleteUserRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	return nil, trail.ToGRPC(m.err)
}

// reset resets states to its zero values.
func (m *mockAuthServiceClient) reset() {
	m.err = nil
}

type TLSSuite struct {
	dataDir string
	server  *TestTLSServer
}

var _ = check.Suite(&TLSSuite{})
var _ = testing.Verbose
var _ = fmt.Printf

func (s *TLSSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests(testing.Verbose())
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
	c.Assert(err, check.IsNil)

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
	var ok bool

	remoteServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:         c.MkDir(),
		ClusterName: "remote",
	})
	c.Assert(err, check.IsNil)

	certPool, err := s.server.CertPool()
	c.Assert(err, check.IsNil)

	// after trust is established, things are good
	err = s.server.AuthServer.Trust(remoteServer, nil)
	c.Assert(err, check.IsNil)

	remoteProxy, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleProxy), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	remoteAuth, err := remoteServer.NewRemoteClient(
		TestBuiltin(teleport.RoleAuth), s.server.Addr(), certPool)
	c.Assert(err, check.IsNil)

	// remote cluster starts rotation
	gracePeriod := time.Hour
	remoteServer.AuthServer.privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
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

	// remote proxy can't read local cert authority with secrets
	_, err = remoteProxy.GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, true)
	fixtures.ExpectAccessDenied(c, err)

	// no secrets read is allowed
	_, err = remoteProxy.GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
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
	var ok bool

	clock := clockwork.NewFakeClockAt(time.Now().Add(-2 * time.Hour))
	s.server.Auth().SetClock(clock)

	// create proxy client
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// starts rotation
	s.server.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
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
	_, err = s.server.NewClient(TestBuiltin(teleport.RoleProxy))
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
	newProxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	_, err = newProxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// complete rotation - advance rotation by clock
	clock.Advance(gracePeriod/3 + time.Minute)
	err = s.server.Auth().autoRotateCertAuthorities()
	c.Assert(err, check.IsNil)
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
	var ok bool

	clock := clockwork.NewFakeClockAt(time.Now().Add(-2 * time.Hour))
	s.server.Auth().SetClock(clock)

	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// starts rotation
	s.server.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
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
	var ok bool

	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// can't jump to mid-phase
	gracePeriod := time.Hour
	s.server.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
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
	var ok bool

	// create proxy client just for test purposes
	proxy, err := s.server.NewClient(TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	// client works before rotation is initiated
	_, err = proxy.GetNodes(defaults.Namespace, services.SkipValidation())
	c.Assert(err, check.IsNil)

	// starts rotation
	gracePeriod := time.Hour
	s.server.Auth().privateKey, ok = fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)
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
	_, err = client.GetUsers(false)
	fixtures.ExpectAccessDenied(c, err)

	_, err = client.GetNodes(defaults.Namespace, services.SkipValidation())
	fixtures.ExpectAccessDenied(c, err)

	// Endpoints that allow current user access should return access denied to
	// the Nop user.
	err = client.CheckPassword("foo", nil, "")
	fixtures.ExpectAccessDenied(c, err)
}

// TestOwnRole tests that user can read roles assigned to them (used by web UI)
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

	testSuite := &suite.ServicesTestSuite{
		ConfigS: clt,
	}
	testSuite.ClusterConfig(c, suite.SkipDelete())
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

	err = s.server.Auth().UpsertTOTP("user1", otpSecret)
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

	out, err := clt.GenerateToken(ctx, GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleNode}})
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Not(check.Equals), 0)
}

func (s *TLSSuite) TestValidateUploadSessionRecording(c *check.C) {
	serverID, err := s.server.Identity.ID.HostID()
	c.Assert(err, check.IsNil)

	var tests = []struct {
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
		clt, err := s.server.NewClient(TestServerID(serverID))
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
			Namespace:      defaults.Namespace,
		}
		c.Assert(clt.CreateSession(sess), check.IsNil)

		err = clt.UploadSessionRecording(events.SessionRecording{
			Namespace: defaults.Namespace,
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

	var tests = []struct {
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
		clt, err := s.server.NewClient(TestServerID(serverID))
		c.Assert(err, check.IsNil)

		date := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
		sess := session.Session{
			ID:             session.NewID(),
			TerminalParams: session.TerminalParams{W: 100, H: 100},
			Created:        date,
			LastActive:     date,
			Login:          "bob",
			Namespace:      defaults.Namespace,
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
			Namespace: defaults.Namespace,
			SessionID: string(sess.ID),
			Chunks: []*events.SessionChunk{
				&events.SessionChunk{
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

	out, err := clt.GetSessions(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.DeepEquals, []session.Session{})

	date := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	sess := session.Session{
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
	c.Assert(err, check.IsNil)
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
	c.Assert(err, check.IsNil)

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
	err = web.DeleteUser(context.TODO(), user)
	fixtures.ExpectAccessDenied(c, err)

	err = clt.DeleteWebSession(user, ws.GetName())
	c.Assert(err, check.IsNil)

	_, err = web.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, check.NotNil)

	_, err = web.ExtendWebSession(user, ws.GetName())
	c.Assert(err, check.NotNil)
}

// TestGetCertAuthority tests certificate authority permissions
func (s *TLSSuite) TestGetCertAuthority(c *check.C) {
	ctx := context.Background()
	// generate server keys for node
	nodeClt, err := s.server.NewClient(TestIdentity{I: BuiltinRole{Username: "00000000-0000-0000-0000-000000000000", Role: teleport.RoleNode}})
	c.Assert(err, check.IsNil)
	defer nodeClt.Close()

	// node is authorized to fetch CA without secrets
	ca, err := nodeClt.GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)
	for _, keyPair := range ca.GetTLSKeyPairs() {
		c.Assert(keyPair.Key, check.IsNil)
	}
	keys := ca.GetSigningKeys()
	c.Assert(keys, check.HasLen, 0)

	// node is not authorized to fetch CA with secrets
	_, err = nodeClt.GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, true)
	fixtures.ExpectAccessDenied(c, err)

	// non-admin users are not allowed to get access to private key material
	user, err := services.NewUser("bob")
	c.Assert(err, check.IsNil)

	role := services.RoleForUser(user)
	role.SetLogins(services.Allow, []string{user.GetName()})
	err = s.server.Auth().UpsertRole(ctx, role)
	c.Assert(err, check.IsNil)

	user.AddRole(role.GetName())
	err = s.server.Auth().UpsertUser(user)
	c.Assert(err, check.IsNil)

	userClt, err := s.server.NewClient(TestUser(user.GetName()))
	c.Assert(err, check.IsNil)
	defer userClt.Close()

	// user is authorized to fetch CA without secrets
	_, err = userClt.GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	// user is not authorized to fetch CA with secrets
	_, err = userClt.GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
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

	user := "user1"
	role := "some-role"
	_, err = CreateUserRoleAndRequestable(s.server.Auth(), user, role)
	c.Assert(err, check.IsNil)

	testUser := TestUser(user)
	testUser.TTL = time.Hour
	userClient, err := s.server.NewClient(testUser)
	c.Assert(err, check.IsNil)

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
			Format:         teleport.CertificateFormatStandard,
			AccessRequests: reqIDs,
		})
	}

	// certContainsRole checks if a PEM encoded TLS cert contains the
	// specified role.
	certContainsRole := func(cert []byte, role string) bool {
		tlsCert, err := tlsca.ParseCertificatePEM(cert)
		c.Assert(err, check.IsNil)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		c.Assert(err, check.IsNil)
		return utils.SliceContainsStr(identity.Groups, role)
	}

	// sanity check; ensure that role is not held if no request is applied.
	userCerts, err := generateCerts()
	c.Assert(err, check.IsNil)
	if certContainsRole(userCerts.TLS, role) {
		c.Errorf("unexpected role %s", role)
	}

	// attempt to apply request in PENDING state (should fail)
	_, err = generateCerts(req.GetName())
	c.Assert(err, check.NotNil)

	ctx := context.Background()

	// verify that user does not have the ability to approve their own request (not a special case, this
	// user just wasn't created with the necessary roles for request management).
	c.Assert(userClient.SetAccessRequestState(ctx, req.GetName(), services.RequestState_APPROVED), check.NotNil)

	// attempt to apply request in APPROVED state (should succeed)
	c.Assert(s.server.Auth().SetAccessRequestState(ctx, req.GetName(), services.RequestState_APPROVED), check.IsNil)
	userCerts, err = generateCerts(req.GetName())
	c.Assert(err, check.IsNil)
	// ensure that the requested role was actually applied to the cert
	if !certContainsRole(userCerts.TLS, role) {
		c.Errorf("missing requested role %s", role)
	}

	// attempt to apply request in DENIED state (should fail)
	c.Assert(s.server.Auth().SetAccessRequestState(ctx, req.GetName(), services.RequestState_DENIED), check.IsNil)
	_, err = generateCerts(req.GetName())
	c.Assert(err, check.NotNil)

	// ensure that once in the DENIED state, a request cannot be set back to PENDING state.
	c.Assert(s.server.Auth().SetAccessRequestState(ctx, req.GetName(), services.RequestState_PENDING), check.NotNil)

	// ensure that once in the DENIED state, a request cannot be set back to APPROVED state.
	c.Assert(s.server.Auth().SetAccessRequestState(ctx, req.GetName(), services.RequestState_APPROVED), check.NotNil)
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

	req, err := services.NewAccessRequest(user, role)
	c.Assert(err, check.IsNil)

	c.Assert(userClient.CreateAccessRequest(context.TODO(), req), check.IsNil)

	plugin := "my-plugin"

	err = s.server.Auth().UpdatePluginData(context.TODO(), services.PluginDataUpdateParams{
		Kind:     services.KindAccessRequest,
		Resource: req.GetName(),
		Plugin:   plugin,
		Set: map[string]string{
			"foo": "bar",
		},
	})
	c.Assert(err, check.IsNil)

	data, err := s.server.Auth().GetPluginData(context.TODO(), services.PluginDataFilter{
		Kind:     services.KindAccessRequest,
		Resource: req.GetName(),
	})
	c.Assert(err, check.IsNil)
	c.Assert(len(data), check.Equals, 1)

	entry, ok := data[0].Entries()[plugin]
	c.Assert(ok, check.Equals, true)
	c.Assert(entry.Data, check.DeepEquals, map[string]string{"foo": "bar"})

	err = s.server.Auth().UpdatePluginData(context.TODO(), services.PluginDataUpdateParams{
		Kind:     services.KindAccessRequest,
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

	data, err = s.server.Auth().GetPluginData(context.TODO(), services.PluginDataFilter{
		Kind:     services.KindAccessRequest,
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
func (s *TLSSuite) TestGenerateCerts(c *check.C) {
	ctx := context.Background()
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

	_, err = nopClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: pub,
		Username:  user1.GetName(),
		Expires:   time.Now().Add(time.Hour).UTC(),
		Format:    teleport.CertificateFormatStandard,
	})
	c.Assert(err, check.NotNil)
	fixtures.ExpectAccessDenied(c, err)
	c.Assert(err, check.ErrorMatches, "this request can be only executed by an admin")

	// User can't generate certificates for another user
	testUser2 := TestUser(user2.GetName())
	testUser2.TTL = time.Hour
	userClient2, err := s.server.NewClient(testUser2)
	c.Assert(err, check.IsNil)

	_, err = userClient2.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: pub,
		Username:  user1.GetName(),
		Expires:   time.Now().Add(time.Hour).UTC(),
		Format:    teleport.CertificateFormatStandard,
	})
	c.Assert(err, check.NotNil)
	fixtures.ExpectAccessDenied(c, err)
	c.Assert(err, check.ErrorMatches, "this request can be only executed by an admin")

	// User can renew their certificates, however the TTL will be limited
	// to the TTL of their session for both SSH and x509 certs and
	// that route to cluster will be encoded in the cert metadata
	userCerts, err := userClient2.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey:      pub,
		Username:       user2.GetName(),
		Expires:        time.Now().Add(100 * time.Hour).UTC(),
		Format:         teleport.CertificateFormatStandard,
		RouteToCluster: "cluster1",
	})
	c.Assert(err, check.IsNil)

	parseCert := func(sshCert []byte) (*ssh.Certificate, time.Duration) {
		parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(sshCert)
		c.Assert(err, check.IsNil)
		parsedCert, _ := parsedKey.(*ssh.Certificate)
		validBefore := time.Unix(int64(parsedCert.ValidBefore), 0)
		return parsedCert, time.Until(validBefore)
	}
	_, diff := parseCert(userCerts.SSH)
	c.Assert(diff < testUser2.TTL, check.Equals, true, check.Commentf("expected %v < %v", diff, testUser2.TTL))

	tlsCert, err := tlsca.ParseCertificatePEM(userCerts.TLS)
	c.Assert(err, check.IsNil)
	identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
	c.Assert(err, check.IsNil)
	c.Assert(identity.Expires.Before(time.Now().Add(testUser2.TTL)), check.Equals, true, check.Commentf("%v vs %v", identity.Expires, time.Now().UTC()))
	c.Assert(identity.RouteToCluster, check.Equals, "cluster1")

	// Admin should be allowed to generate certs with TTL longer than max.
	adminClient, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	userCerts, err = adminClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: pub,
		Username:  user1.GetName(),
		Expires:   time.Now().Add(40 * time.Hour).UTC(),
		Format:    teleport.CertificateFormatStandard,
	})
	c.Assert(err, check.IsNil)

	parsedCert, diff := parseCert(userCerts.SSH)
	c.Assert(diff > defaults.MaxCertDuration, check.Equals, true, check.Commentf("expected %v > %v", diff, defaults.CertDuration))

	// user should have agent forwarding (default setting)
	_, exists := parsedCert.Extensions[teleport.CertExtensionPermitAgentForwarding]
	c.Assert(exists, check.Equals, true)

	// user should not have X11 forwarding (default setting)
	_, exists = parsedCert.Extensions[teleport.CertExtensionPermitX11Forwarding]
	c.Assert(exists, check.Equals, false)

	// now update role to permit agent and X11 forwarding
	roleOptions := userRole.GetOptions()
	roleOptions.ForwardAgent = services.NewBool(true)
	roleOptions.PermitX11Forwarding = services.NewBool(true)
	userRole.SetOptions(roleOptions)
	err = s.server.Auth().UpsertRole(ctx, userRole)
	c.Assert(err, check.IsNil)

	userCerts, err = adminClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: pub,
		Username:  user1.GetName(),
		Expires:   time.Now().Add(1 * time.Hour).UTC(),
		Format:    teleport.CertificateFormatStandard,
	})
	c.Assert(err, check.IsNil)
	parsedCert, _ = parseCert(userCerts.SSH)

	// user should get agent forwarding
	_, exists = parsedCert.Extensions[teleport.CertExtensionPermitAgentForwarding]
	c.Assert(exists, check.Equals, true)

	// user should get X11 forwarding
	_, exists = parsedCert.Extensions[teleport.CertExtensionPermitX11Forwarding]
	c.Assert(exists, check.Equals, true)

	// apply HTTP Auth to generate user cert:
	userCerts, err = adminClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: pub,
		Username:  user1.GetName(),
		Expires:   time.Now().Add(time.Hour).UTC(),
		Format:    teleport.CertificateFormatStandard,
	})
	c.Assert(err, check.IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(userCerts.SSH)
	c.Assert(err, check.IsNil)
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
		err := s.server.Auth().UpsertRole(ctx, userRole)
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

func (s *TLSSuite) TestChangePasswordWithToken(c *check.C) {
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		LocalAuth: services.NewBool(true),
	})
	c.Assert(err, check.IsNil)

	err = s.server.Auth().SetClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, check.IsNil)

	err = s.server.Auth().SetAuthPreference(authPreference)
	c.Assert(err, check.IsNil)

	username := "user1"
	// Create a local user.
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	_, _, err = CreateUserAndRole(clt, username, []string{"role1"})
	c.Assert(err, check.IsNil)

	token, err := s.server.Auth().CreateResetPasswordToken(context.TODO(), CreateResetPasswordTokenRequest{
		Name: username,
		TTL:  time.Hour,
	})
	c.Assert(err, check.IsNil)

	secrets, err := s.server.Auth().RotateResetPasswordTokenSecrets(context.TODO(), token.GetName())
	c.Assert(err, check.IsNil)

	otpToken, err := totp.GenerateCode(secrets.GetOTPKey(), s.server.Clock().Now())
	c.Assert(err, check.IsNil)

	_, err = s.server.Auth().ChangePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:           token.GetName(),
		Password:          []byte("qweqweqwe"),
		SecondFactorToken: otpToken,
	})
	c.Assert(err, check.IsNil)
}

// TestLoginNoLocalAuth makes sure that logins for local accounts can not be
// performed when local auth is disabled.
func (s *TLSSuite) TestLoginNoLocalAuth(c *check.C) {
	user := "foo"
	pass := []byte("barbaz")

	// Create a local user.
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)
	_, _, err = CreateUserAndRole(clt, user, []string{user})
	c.Assert(err, check.IsNil)
	err = clt.UpsertPassword(user, pass)
	c.Assert(err, check.IsNil)

	// Set services.ClusterConfig to disallow local auth.
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		LocalAuth: services.NewBool(false),
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetClusterConfig(clusterConfig)
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

	addrs := []utils.NetAddr{
		utils.FromAddr(otherServer.Listener.Addr()),
		utils.FromAddr(s.server.Listener.Addr()),
	}
	client, err := NewTLSClient(ClientConfig{
		Addrs: addrs,
		TLS:   tlsConfig,
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

	addrs := []utils.NetAddr{
		utils.FromAddr(otherServer.Listener.Addr()),
		utils.FromAddr(s.server.Listener.Addr()),
	}
	client, err := NewTLSClient(ClientConfig{Addrs: addrs, TLS: tlsConfig})
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
	caPin := utils.CalculateSPKI(tlsCA.Cert)

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
	ctx := context.Background()
	// Generate a token to use.
	token, err := s.server.AuthServer.AuthServer.GenerateToken(ctx, GenerateTokenRequest{
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

// TestEventsNodePresence tests streaming node presence API -
// announcing node and keeping node alive
func (s *TLSSuite) TestEventsNodePresence(c *check.C) {
	node := &services.ServerV2{
		Kind:    services.KindNode,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      "node1",
			Namespace: defaults.Namespace,
		},
		Spec: services.ServerSpecV2{
			Addr: "localhost:3022",
		},
	}
	node.SetExpiry(time.Now().Add(2 * time.Second))
	clt, err := s.server.NewClient(TestIdentity{
		I: BuiltinRole{
			Role:     teleport.RoleNode,
			Username: fmt.Sprintf("%v.%v", node.Metadata.Name, s.server.ClusterName()),
		},
	})
	c.Assert(err, check.IsNil)
	defer clt.Close()

	keepAlive, err := clt.UpsertNode(node)
	c.Assert(err, check.IsNil)
	c.Assert(keepAlive, check.NotNil)

	ctx := context.TODO()
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
	nopClt, err := s.server.NewClient(TestBuiltin(teleport.RoleNop))
	c.Assert(err, check.IsNil)
	defer nopClt.Close()

	_, err = nopClt.UpsertNode(node)
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
	clt, err := s.server.NewClient(TestBuiltin(teleport.RoleNode))
	c.Assert(err, check.IsNil)
	defer clt.Close()

	ctx := context.TODO()
	w, err := clt.NewWatcher(ctx, services.Watch{Kinds: []services.WatchKind{{Kind: services.KindCertAuthority}}})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for init event")
	case event := <-w.Events():
		c.Assert(event.Type, check.Equals, backend.OpInit)
	}

	// start rotation
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, false)
	c.Assert(err, check.IsNil)

	suite.ExpectResource(c, w, 3*time.Second, ca)

	type testCase struct {
		name     string
		identity TestIdentity
		watches  []services.WatchKind
	}

	testCases := []testCase{
		{
			name:     "node role is not authorized to get certificate authority with secret data loaded",
			identity: TestBuiltin(teleport.RoleNode),
			watches:  []services.WatchKind{{Kind: services.KindCertAuthority, LoadSecrets: true}},
		},
		{
			name:     "node role is not authorized to watch static tokens",
			identity: TestBuiltin(teleport.RoleNode),
			watches:  []services.WatchKind{{Kind: services.KindStaticTokens}},
		},
		{
			name:     "node role is not authorized to watch provisioning tokens",
			identity: TestBuiltin(teleport.RoleNode),
			watches:  []services.WatchKind{{Kind: services.KindToken}},
		},
		{
			name:     "nop role is not authorized to watch users and roles",
			identity: TestBuiltin(teleport.RoleNop),
			watches: []services.WatchKind{
				{Kind: services.KindUser},
				{Kind: services.KindRole},
			},
		},
		{
			name:     "nop role is not authorized to watch cert authorities",
			identity: TestBuiltin(teleport.RoleNop),
			watches:  []services.WatchKind{{Kind: services.KindCertAuthority, LoadSecrets: false}},
		},
		{
			name:     "nop role is not authorized to watch cluster config",
			identity: TestBuiltin(teleport.RoleNop),
			watches:  []services.WatchKind{{Kind: services.KindClusterConfig, LoadSecrets: false}},
		},
	}

	tryWatch := func(tc testCase) {
		client, err := s.server.NewClient(tc.identity)
		c.Assert(err, check.IsNil)
		defer client.Close()

		watcher, err := client.NewWatcher(ctx, services.Watch{
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

// TestEventsClusterConifg test cluster configuration
func (s *TLSSuite) TestEventsClusterConfig(c *check.C) {
	clt, err := s.server.NewClient(TestBuiltin(teleport.RoleAdmin))
	c.Assert(err, check.IsNil)
	defer clt.Close()

	ctx := context.TODO()
	w, err := clt.NewWatcher(ctx, services.Watch{Kinds: []services.WatchKind{
		{Kind: services.KindCertAuthority, LoadSecrets: true},
		{Kind: services.KindStaticTokens},
		{Kind: services.KindToken},
		{Kind: services.KindClusterConfig},
		{Kind: services.KindClusterName},
	}})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for init event")
	case event := <-w.Events():
		c.Assert(event.Type, check.Equals, backend.OpInit)
	}

	// start rotation
	gracePeriod := time.Hour
	err = s.server.Auth().RotateCertAuthority(RotateRequest{
		Type:        services.HostCA,
		GracePeriod: &gracePeriod,
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	ca, err := s.server.Auth().GetCertAuthority(services.CertAuthID{
		DomainName: s.server.ClusterName(),
		Type:       services.HostCA,
	}, true)
	c.Assert(err, check.IsNil)

	suite.ExpectResource(c, w, 3*time.Second, ca)

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{
			{
				Token:   "tok1",
				Roles:   teleport.Roles{teleport.RoleNode},
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
	token, err := services.NewProvisionToken(
		"tok2", teleport.Roles{teleport.RoleProxy}, time.Now().UTC().Add(3*time.Hour))
	c.Assert(err, check.IsNil)

	err = s.server.Auth().UpsertToken(token)
	c.Assert(err, check.IsNil)

	token, err = s.server.Auth().GetToken(token.GetName())
	c.Assert(err, check.IsNil)

	suite.ExpectResource(c, w, 3*time.Second, token)

	// delete token and expect delete event
	err = s.server.Auth().DeleteToken(token.GetName())
	c.Assert(err, check.IsNil)
	suite.ExpectDeleteResource(c, w, 3*time.Second, &services.ResourceHeader{
		Kind:    services.KindToken,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      token.GetName(),
		},
	})

	// update cluster config to record at the proxy
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording: services.RecordAtProxy,
		Audit: services.AuditConfig{
			AuditEventsURI: []string{"dynamodb://audit_table_name", "file:///home/log"},
		},
	})
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	clusterConfig, err = s.server.Auth().GetClusterConfig()
	c.Assert(err, check.IsNil)
	suite.ExpectResource(c, w, 3*time.Second, clusterConfig)

	// update cluster name resource metadata
	clusterNameResource, err := s.server.Auth().GetClusterName()
	c.Assert(err, check.IsNil)

	// update the resource with different labels to test the change
	clusterName := &services.ClusterNameV2{
		Kind:    services.KindClusterName,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      services.MetaNameClusterName,
			Namespace: defaults.Namespace,
			Labels: map[string]string{
				"key": "val",
			},
		},
		Spec: services.ClusterNameSpecV2{
			ClusterName: clusterNameResource.GetClusterName(),
		},
	}

	err = s.server.Auth().DeleteClusterName()
	c.Assert(err, check.IsNil)
	err = s.server.Auth().SetClusterName(clusterName)
	c.Assert(err, check.IsNil)

	clusterNameResource, err = s.server.Auth().ClusterConfiguration.GetClusterName()
	c.Assert(err, check.IsNil)
	suite.ExpectResource(c, w, 3*time.Second, clusterNameResource)
}

func (s *TLSSuite) TestAPIBackwardsCompatibilityLogic(c *check.C) {
	clt, err := s.server.NewClient(TestAdmin())
	c.Assert(err, check.IsNil)

	grpcMock := newMockAuthServiceClient()
	clt.grpcClient = grpcMock

	// check the REST endpoint gets called
	grpcMock.err = trace.NotImplemented("")
	err = clt.DeleteUser(context.TODO(), "not-existing-user")
	c.Assert(trace.IsNotFound(err), check.Equals, true)
	grpcMock.reset()

	// check that any other error than "NotImplemented" returns right away
	grpcMock.err = trace.BadParameter("test-other-error")
	err = clt.DeleteUser(context.TODO(), "not-existing-user")
	c.Assert(trace.IsBadParameter(err), check.Equals, true)
	c.Assert(err, check.ErrorMatches, `test-other-error`)
}
