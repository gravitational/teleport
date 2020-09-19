/*
Copyright 2020 Gravitational, Inc.

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

// app package runs the AAP server process. It keeps dynamic labels updated,
// heart beat it's presence, and forward connections between the tunnel and
// the target host.
package app

import (
	"context"
	"crypto/x509"
	"io"
	"math/rand"
	"net"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

type RotationGetter func(role teleport.Role) (*services.Rotation, error)

type Request struct {
	// Conn is a connection between the reverse tunnel and application node.
	Conn net.Conn

	// PublicAddr is the name of the application being requested.
	PublicAddr string

	// Certificate is a PEM encoded x509 certificate that contains the identity
	// of the requester.
	Certificate []byte
}

// Config is the configuration for an application server.
type Config struct {
	// Clock used to control time.
	Clock clockwork.Clock

	// AccessPoint is a client connected to the Auth Server with the identity
	// teleport.RoleApp.
	AccessPoint auth.AccessPoint

	// GetRotation returns the certificate rotation state.
	GetRotation RotationGetter

	// Server contains the list of applications that will be proxied.
	Server services.Server
}

// CheckAndSetDefaults makes sure the configuration has the minimum required
// to function.
func (c *Config) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.AccessPoint == nil {
		return trace.BadParameter("access point is missing")
	}
	if c.GetRotation == nil {
		return trace.BadParameter("rotation getter is missing")
	}
	if c.Server == nil {
		return trace.BadParameter("server is missing")
	}
	return nil
}

// Server is an application server.
type Server struct {
	c   *Config
	log *logrus.Entry

	closeContext context.Context
	closeFunc    context.CancelFunc

	heartbeat     *srv.Heartbeat
	dynamicLabels map[string]*labels.Dynamic
	clusterName   string

	keepAlive time.Duration

	activeConns int64
}

// New returns a new application server.
func New(ctx context.Context, c *Config) (*Server, error) {
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Server{
		c: c,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentApp,
		}),
	}

	s.closeContext, s.closeFunc = context.WithCancel(ctx)

	// Create dynamic labels for all applications that are being proxied and
	// sync them right away so the first heartbeat has correct dynamic labels.
	s.dynamicLabels = make(map[string]*labels.Dynamic)
	for _, a := range c.Server.GetApps() {
		if len(a.DynamicLabels) == 0 {
			continue
		}
		dl, err := labels.NewDynamic(s.closeContext, &labels.DynamicConfig{
			Labels: services.V2ToLabels(a.DynamicLabels),
			Log:    s.log,
		})
		if err != nil {

			return nil, trace.Wrap(err)
		}
		dl.Sync()
		s.dynamicLabels[a.Name] = dl
	}

	// Create heartbeat loop so applications keep sending presence to backend.
	s.heartbeat, err = srv.NewHeartbeat(srv.HeartbeatConfig{
		Mode:            srv.HeartbeatModeApp,
		Context:         s.closeContext,
		Component:       teleport.ComponentApp,
		Announcer:       c.AccessPoint,
		GetServerInfo:   s.GetServerInfo,
		KeepAlivePeriod: defaults.ServerKeepAliveTTL,
		AnnouncePeriod:  defaults.ServerAnnounceTTL/2 + utils.RandomDuration(defaults.ServerAnnounceTTL/2),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       defaults.ServerAnnounceTTL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Pick up TCP keep-alive settings from the cluster level.
	clusterConfig, err := s.c.AccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.keepAlive = clusterConfig.GetKeepAliveInterval()

	cn, err := s.c.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.clusterName = cn.GetClusterName()

	return s, nil
}

// GetServerInfo returns a services.Server representing the application. Used
// in heartbeat code.
func (s *Server) GetServerInfo() (services.Server, error) {
	// Update dynamic labels on all apps.
	apps := s.c.Server.GetApps()
	for _, a := range apps {
		dl, ok := s.dynamicLabels[a.Name]
		if !ok {
			continue
		}
		a.DynamicLabels = services.LabelsToV2(dl.Get())
	}
	s.c.Server.SetApps(apps)

	// Update the TTL.
	s.c.Server.SetTTL(s.c.Clock, defaults.ServerAnnounceTTL)

	// Update rotation state.
	rotation, err := s.c.GetRotation(teleport.RoleApp)
	if err != nil {
		if !trace.IsNotFound(err) {
			s.log.Warningf("Failed to get rotation state: %v.", err)
		}
	} else {
		s.c.Server.SetRotation(*rotation)
	}

	return s.c.Server, nil
}

// Start starts heart beating the presence of service.Apps that this
// server is proxying along with any dynamic labels.
func (s *Server) Start() {
	for _, dynamicLabel := range s.dynamicLabels {
		go dynamicLabel.Start()
	}
	go s.heartbeat.Run()
}

// Serve accepts incoming connections on the Listener and calls the handler.
func (s *Server) HandleConnection(ctx context.Context, r *Request) {
	channelConn := r.Conn

	// Before forwarding the request to the target application, check if the
	// caller has access to this application.
	application, err := s.checkAccess(ctx, r.Certificate, r.PublicAddr)
	if err != nil {
		s.log.Errorf("Denied access to %v: %v.", r.PublicAddr, err)
		channelConn.Close()
		return
	}

	u, err := url.Parse(application.URI)
	if err != nil {
		s.log.Errorf("Application running at invalid address: %v: %v.", application.URI, err)
		channelConn.Close()
		return
	}

	// Establish connection to target server.
	//s.log.Debugf("Dialing to %v.", application.)
	d := net.Dialer{
		KeepAlive: s.keepAlive,
	}
	targetConn, err := d.DialContext(s.closeContext, "tcp", u.Host)
	if err != nil {
		s.log.Errorf("Failed to connect to %v: %v.", u.Host, err)
		channelConn.Close()
		return
	}

	// Keep a count of the number of active connections. Used in tests to check
	// for goroutine leaks.
	atomic.AddInt64(&s.activeConns, 1)
	defer atomic.AddInt64(&s.activeConns, -1)

	errorCh := make(chan error, 2)

	// Copy data between channel connection and connection to target application.
	go func() {
		defer targetConn.Close()
		defer channelConn.Close()

		_, err := io.Copy(targetConn, channelConn)
		errorCh <- err
	}()
	go func() {
		defer targetConn.Close()
		defer channelConn.Close()

		_, err := io.Copy(channelConn, targetConn)
		errorCh <- err
	}()

	// Block until connection is closed.
	for i := 0; i < 2; i++ {
		select {
		case err := <-errorCh:
			if err != nil && err != io.EOF {
				s.log.Debugf("Proxy transport failed: %v.", err)
			}
		}
	}
}

func (s *Server) ForceHeartbeat() error {
	err := s.heartbeat.ForceSend(time.Second)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close will shut the server down and unblock any resources.
func (s *Server) Close() error {
	err := s.heartbeat.Close()
	for _, dynamicLabel := range s.dynamicLabels {
		dynamicLabel.Close()
	}
	s.closeFunc()

	return trace.Wrap(err)
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.closeContext.Done()
	return s.closeContext.Err()
}

func (s *Server) GetApp(ctx context.Context, publicAddr string) (*services.App, error) {
	var matches []*services.App

	servers, err := s.c.AccessPoint.GetApps(ctx, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, server := range servers {
		for _, a := range server.GetApps() {
			if publicAddr == a.PublicAddr {
				matches = append(matches, a)
			}
		}
	}

	if len(matches) == 0 {
		return nil, trace.NotFound("no application at %v found", publicAddr)
	}
	return matches[rand.Intn(len(matches))], nil
}

func (s *Server) checkAccess(ctx context.Context, certBytes []byte, publicAddr string) (*services.App, error) {
	// Verify and extract the identity of the caller.
	identity, ca, err := s.verifyCertificate(certBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Find the application the caller is requesting by public address.
	app, err := s.GetApp(ctx, publicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build the access checker either directly or by mapping roles depending on
	// if this code is running within the same cluster that issued the identity
	// or if it's running in a leaf cluster.
	checker, err := s.buildChecker(identity, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if the caller has access to the application being requested.
	err = checker.CheckAccessToApp(app)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return app, nil
}

func (s *Server) buildChecker(identity *tlsca.Identity, ca services.CertAuthority) (services.AccessChecker, error) {
	var checker services.AccessChecker

	// If the caller has an identity issued the same cluster that the application
	// proxy is running in, directly build the access checker. Otherwise map the
	// roles, then build the access checker.
	if s.clusterName == ca.GetClusterName() {
		roles, traits, err := services.ExtractFromIdentity(s.c.AccessPoint, identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		checker, err = services.FetchRoles(roles, s.c.AccessPoint, traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		roleNames, err := ca.CombinedMapping().Map(identity.Groups)
		if err != nil {
			return nil, trace.AccessDenied("failed to map roles")
		}
		// Pass the principals on the certificate along as the login traits
		// to the remote cluster.
		//traits := map[string][]string{
		//	teleport.TraitLogins: cert.ValidPrincipals,
		//}
		checker, err = services.FetchRoles(roleNames, s.c.AccessPoint, map[string][]string{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return checker, nil
}

func (s *Server) verifyCertificate(bytes []byte) (*tlsca.Identity, services.CertAuthority, error) {
	// Parse certificate and extract the name of the cluster the certificate
	// claims it was issued by.
	cert, err := tlsca.ParseCertificatePEM(bytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clusterName, err := tlsca.ClusterName(cert.Issuer)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Find the CA the certificate was signed by.
	ca, err := s.c.AccessPoint.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Verify the CA signed the certificate.
	roots := x509.NewCertPool()
	for _, keyPair := range ca.GetTLSKeyPairs() {
		ok := roots.AppendCertsFromPEM(keyPair.Cert)
		if !ok {
			return nil, nil, trace.BadParameter("failed to create certificate pool: %v", err)
		}
	}
	_, err = cert.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Now that it's been verified that a CA that this cluster trusts was the
	// one to sign the certificate, extract and return the identity.
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return identity, ca, nil
}

// activeConnections returns the number of active connections being proxied.
// Used in tests.
func (s *Server) activeConnections() int64 {
	return atomic.LoadInt64(&s.activeConns)
}
