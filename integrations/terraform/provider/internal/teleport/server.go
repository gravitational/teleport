// Copyright 2026 Gravitational, Inc.
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

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewServerClient returns a Teleport client to interact with SSH server resources.
func NewServerClient(c *client.Client) ServerClient {
	return ServerClient{client: c}
}

// ServerClient manages SSH server resources in Teleport.
type ServerClient struct {
	client *client.Client
}

// Get reads an SSH server by scope-qualified name.
func (r ServerClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*types.ServerV2, error) {
	server, err := r.client.GetSSHServer(ctx, presencev1.GetSSHServerRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverV2, ok := server.(*types.ServerV2)
	if !ok {
		return nil, trace.BadParameter("unexpected SSH server type: %T", server)
	}

	return serverV2, nil
}

// Create creates an SSH server resource in Teleport.
func (r ServerClient) Create(ctx context.Context, server *types.ServerV2) error {
	_, err := r.client.UpsertNode(ctx, server)
	return trace.Wrap(err)
}

// Upsert updates an SSH server resource in Teleport.
func (r ServerClient) Upsert(ctx context.Context, server *types.ServerV2) error {
	_, err := r.client.UpsertNode(ctx, server)
	return trace.Wrap(err)
}

// Delete deletes an SSH server resource from Teleport by scope-qualified name.
func (r ServerClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	return trace.Wrap(r.client.DeleteSSHServer(ctx, presencev1.DeleteSSHServerRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build()))
}
