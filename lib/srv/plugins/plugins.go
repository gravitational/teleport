/*
Copyright 2023 Gravitational, Inc.

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

package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// Config provides configuration for the plugins server.
type Config struct {
	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter
	// AccessPoint is a plugins access point
	AccessPoint auth.PluginsAccessPoint
	// Log is the logger.
	Log logrus.FieldLogger
}

func (c *Config) CheckAndSetDefaults() error {
	return nil
}

// Server is a plugins server, used to discover cloud resources for
// inclusion in Teleport
type Server struct {
	*Config

	ctx context.Context
	// cancelfn is used with ctx when stopping the plugins server
	cancelfn context.CancelFunc
	// accessRequestWatcher is an access request watcher.
	accessRequestWatcher *services.AccessRequestWatcher
}

// New initializes a plugins Server
func New(ctx context.Context, cfg *Config) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	localCtx, cancelfn := context.WithCancel(ctx)
	s := &Server{
		Config:   cfg,
		ctx:      localCtx,
		cancelfn: cancelfn,
	}
	return s, nil
}

// Start starts the plugins service.
func (s *Server) Start() error {
	return nil
}

// Stop stops the plugins service.
func (s *Server) Stop() {
	s.cancelfn()
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.ctx.Done()
	if err := s.ctx.Err(); err != nil && err != context.Canceled {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) initTeleportAccessRequestWatcher() (err error) {
	s.accessRequestWatcher, err = services.NewAccessRequestWatcher(s.ctx, services.AccessRequestWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:    teleport.ComponentPlugins,
			Log:          s.Log,
			Client:       s.AccessPoint,
			MaxStaleness: time.Minute,
		},
	})

	return trace.Wrap(err)
}
