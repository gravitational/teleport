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
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

type RotationGetter func(role teleport.Role) (*services.Rotation, error)

// Config is the configuration for an application server.
type Config struct {
	// Clock used to control time.
	Clock clockwork.Clock

	// AccessPoint is a client connected to the Auth Server with the identity
	// teleport.RoleApp.
	AccessPoint auth.AccessPoint

	// GetRotation returns the certificate rotation state.
	GetRotation RotationGetter

	// App is the application this server will proxy.
	App services.Server
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
	if c.App == nil {
		return trace.BadParameter("app is missing")
	}
	return nil
}

// Server is an application server.
type Server struct {
	config *Config

	log          *logrus.Entry
	closeContext context.Context
	closeFunc    context.CancelFunc

	dynamicLabels *labels.Dynamic
	heartbeat     *srv.Heartbeat

	keepAlive time.Duration

	activeConns int64
}

// New returns a new application server.
func New(ctx context.Context, config *Config) (*Server, error) {
	componentName := fmt.Sprintf("%v.%v", teleport.ComponentApp, config.App.GetName())

	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Server{
		config: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: componentName,
		}),
	}

	s.closeContext, s.closeFunc = context.WithCancel(ctx)

	// Create dynamic labels and sync them right away. This makes sure that the
	// first heartbeat has correct dynamic labels.
	s.dynamicLabels, err = labels.NewDynamic(s.closeContext, &labels.DynamicConfig{
		Labels: config.App.GetCmdLabels(),
		Log:    s.log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.dynamicLabels.Sync()

	// Create heartbeat loop so applications keep sending presence to backend.
	s.heartbeat, err = srv.NewHeartbeat(srv.HeartbeatConfig{
		Mode:            srv.HeartbeatModeApp,
		Context:         s.closeContext,
		Component:       componentName,
		Announcer:       config.AccessPoint,
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
	clusterConfig, err := s.config.AccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.keepAlive = clusterConfig.GetKeepAliveInterval()

	return s, nil
}

// GetServerInfo returns a services.Server representing the application. Used
// in heartbeat code.
func (s *Server) GetServerInfo() (services.Server, error) {
	// Return a updated list of dynamic labels.
	s.config.App.SetCmdLabels(s.dynamicLabels.Get())

	// Update the TTL.
	s.config.App.SetTTL(s.config.Clock, defaults.ServerAnnounceTTL)

	// Update rotation state.
	rotation, err := s.config.GetRotation(teleport.RoleApp)
	if err != nil {
		if !trace.IsNotFound(err) {
			s.log.Warningf("Failed to get rotation state: %v.", err)
		}
	} else {
		s.config.App.SetRotation(*rotation)
	}

	return s.config.App, nil
}

// Start starts heart beating the presence of service.Apps that this
// server is proxying along with any dynamic labels.
func (s *Server) Start() {
	go s.dynamicLabels.Start()
	go s.heartbeat.Run()
}

// Serve accepts incoming connections on the Listener and calls the handler.
func (s *Server) HandleConnection(channelConn net.Conn) {
	// Establish connection to target server.
	d := net.Dialer{
		KeepAlive: s.keepAlive,
	}
	targetConn, err := d.DialContext(s.closeContext, "tcp", s.config.App.GetInternalAddr())
	if err != nil {
		s.log.Errorf("Failed to connect to %v: %v.", s.config.App.GetName(), err)
		channelConn.Close()
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

	// Block until copy is complete in either direction. The other direction
	// will get cleaned up automatically.
	if err = <-errorCh; err != nil && err != io.EOF {
		s.log.Errorf("Connection to %v closed due to an error: %v.", s.config.App.GetName(), err)
	}
}

// activeConnections returns the number of active connections being proxied.
// Used in tests.
func (s *Server) activeConnections() int64 {
	return atomic.LoadInt64(&s.activeConns)
}

// Close will shut the server down and unblock any resources.
func (s *Server) Close() error {
	err := s.heartbeat.Close()
	s.dynamicLabels.Close()
	s.closeFunc()

	return trace.Wrap(err)
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.closeContext.Done()
	return s.closeContext.Err()
}
