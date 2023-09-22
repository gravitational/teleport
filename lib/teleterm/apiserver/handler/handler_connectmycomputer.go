// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"
	"strings"
	"time"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func (s *Handler) CreateConnectMyComputerRole(ctx context.Context, req *api.CreateConnectMyComputerRoleRequest) (*api.CreateConnectMyComputerRoleResponse, error) {
	res, err := s.DaemonService.CreateConnectMyComputerRole(ctx, req)
	return res, trace.Wrap(err)
}

func (s *Handler) CreateConnectMyComputerNodeToken(ctx context.Context, req *api.CreateConnectMyComputerNodeTokenRequest) (*api.CreateConnectMyComputerNodeTokenResponse, error) {
	token, err := s.DaemonService.CreateConnectMyComputerNodeToken(ctx, req.GetRootClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apiLabels := APILabels{}
	for labelName, labelValues := range token.Labels {
		apiLabels = append(apiLabels, &api.Label{
			Name:  labelName,
			Value: strings.Join(labelValues, " "),
		})
	}

	response := &api.CreateConnectMyComputerNodeTokenResponse{
		Token:  token.Token,
		Labels: apiLabels,
	}

	return response, nil
}

func (s *Handler) DeleteConnectMyComputerToken(ctx context.Context, req *api.DeleteConnectMyComputerTokenRequest) (*api.DeleteConnectMyComputerTokenResponse, error) {
	res, err := s.DaemonService.DeleteConnectMyComputerToken(ctx, req)
	return res, trace.Wrap(err)
}

func (s *Handler) WaitForConnectMyComputerNodeJoin(ctx context.Context, req *api.WaitForConnectMyComputerNodeJoinRequest) (*api.WaitForConnectMyComputerNodeJoinResponse, error) {
	// The Electron app aborts the request after a timeout that's much shorter. However, we're going
	// to add an internal timeout as well to protect from requests hanging forever if a client doesn't
	// set a deadline or doesn't abort the request.
	timeoutCtx, close := context.WithTimeout(ctx, time.Minute)
	defer close()

	rootClusterURI, err := uri.Parse(req.RootClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server, err := s.DaemonService.WaitForConnectMyComputerNodeJoin(timeoutCtx, rootClusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.WaitForConnectMyComputerNodeJoinResponse{
		Server: newAPIServer(server),
	}, err
}
