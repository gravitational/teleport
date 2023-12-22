/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package accessgraph

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

type ServiceConfig struct {
	// Logger is the logger to use.
	Logger logrus.FieldLogger

	// Client is the access graph client.
	Client accessgraphv1.AccessGraphServiceClient

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer
}

type Service struct {
	// client is the access graph client.
	client accessgraphv1.AccessGraphServiceClient

	// authorizer is the authorizer to use.
	authorizer authz.Authorizer

	// log is the logger to use.
	log logrus.FieldLogger

	accessgraphv1.UnimplementedAccessGraphServiceServer
}

func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		client:     cfg.Client,
		log:        cfg.Logger,
		authorizer: cfg.Authorizer,
	}, nil
}

func (c *ServiceConfig) checkAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = logrus.StandardLogger()
	}

	if c.Client == nil {
		return trace.BadParameter("missing access graph client")
	}

	if c.Authorizer == nil {
		return trace.BadParameter("missing authorizer")
	}

	return nil
}

func (s *Service) Query(ctx context.Context, request *accessgraphv1.QueryRequest) (*accessgraphv1.QueryResponse, error) {
	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessGraph, types.VerbRead)
	if authErr != nil {
		return nil, trace.WrapWithMessage(authErr, "not allowed to read the access graph")
	}

	resp, err := s.client.Query(ctx, request)
	return resp, trace.Wrap(err)
}

func (s *Service) GetFile(ctx context.Context, request *accessgraphv1.GetFileRequest) (*accessgraphv1.GetFileResponse, error) {
	// Do not perform access check as we only serve webassets.
	resp, err := s.client.GetFile(ctx, request)
	return resp, trace.Wrap(err)
}

func (s *Service) EventsStream(_ accessgraphv1.AccessGraphService_EventsStreamServer) error {
	return trace.NotImplemented("EventsStream should not be called on the auth server")
}

// RegisterAccessGraphService registers the access graph sync service.
func RegisterAccessGraphService(cfg *servicecfg.Config, process *service.TeleportProcess, license *licensefile.LicenseFile) error {
	if !cfg.AccessGraph.Enabled {
		return nil
	}
	cfg.Log.Debug("Access Graph integration enabled")

	// Register as non-critical service. We don't want to fail the startup if
	// access graph is not available.
	process.RegisterFunc("access-graph-service", func() error {
		// Need to check this here inside the service function, rather than on process creation,
		// since Cloud features are loaded dynamically. More detailed explanation in:
		// https://github.com/gravitational/teleport/blob/3af6d9c1a25836bb160589a27a7d168a19a4992b/lib/service/service.go#L1873
		if !modules.GetModules().Features().IsTeam() {
			cfg.Log.Info("Access Graph specified in config, but license is not for the Team plan. Access Graph sync will not be enabled")
			return nil
		}
		modules.GetModules().EnableAccessGraph()

		cfg.Log.Info("Starting access graph service")

		accessGraphAddr := cfg.AccessGraph.Addr
		if accessGraphAddr == "" {
			cfg.Log.Error("access graph endpoint not configured")
			return trace.NotFound("access graph endpoint not configured")
		}
		const accessGraphRetryPeriod = 5 * time.Second

		ctx := process.GracefulExitContext()
		// TODO(jakule): Very excessive retrying, but we need to make sure that
		// the access graph is initialized before we start serving requests.
		for {
			if err := initializeAndWatchAccessGraph(ctx,
				cfg.Log.WithField("Addr", accessGraphAddr),
				ServiceClientConfig{
					Addr:     accessGraphAddr,
					CA:       cfg.AccessGraph.CA,
					License:  license,
					Insecure: cfg.AccessGraph.Insecure,
				},
				process.GetAuthServer(), process.GetBackend()); err != nil {
				cfg.Log.Errorf("Access graph sync process failed: %v", err)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(accessGraphRetryPeriod):
					continue
				}
			}
		}
	})

	return nil
}
