/*
Copyright 2022 Gravitational, Inc.

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

package client

import (
	"context"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

// SSHConnectionTester implements the ConnectionTester interface for Testing SSH access
type SSHConnectionTester struct {
	clt auth.ClientI
}

// NewSSHConnectionTester creates a new SSHConnectionTester
func NewSSHConnectionTester(clt auth.ClientI) *SSHConnectionTester {
	return &SSHConnectionTester{
		clt: clt,
	}
}

// TestConnection tests an SSH Connection to the target Node using
//  - the provided client
//  - resource name
//  - principal / linux user
// A new ConnectionDiagnostic is created and used to store the traces as it goes through the checkpoints
// To set up the SSH client, it will generate a new cert and inject the ConnectionDiagnosticID
//   - add a trace of whether the SSH Node was reachable
//   - SSH Node receives the cert and extracts the ConnectionDiagnostiID
//   - the SSH Node will append a trace indicating if the has access (RBAC)
//   - the SSH Node will append a trace indicating if the requested principal is valid for the target Node
func (s *SSHConnectionTester) TestConnection(ctx context.Context, req DiagnoseConnectionRequest) (types.ConnectionDiagnostic, error) {
	id := uuid.NewString()
	connDiagInitial, err := types.NewConnectionDiagnosticV1(id, map[string]string{},
		types.ConnectionDiagnosticSpecV1{
			Message: types.MessageWaiting,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.clt.CreateConnectionDiagnostic(ctx, connDiagInitial); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(marco): test ssh connection
	// - create certificate for user with extra field
	// - create ssh client using that certificate
	// - if connection fails because of a network error, a trace must be included indicating that the host is not reachable
	// - other traces will be added by the Node itself (rbac checks, principal)
	connDiagInitial.SetMessage("dry-run")
	connDiagInitial.SetSuccess(true)

	if err := s.clt.UpdateConnectionDiagnostic(ctx, connDiagInitial); err != nil {
		return nil, trace.Wrap(err)
	}

	connectionDiagnostic, err := s.clt.GetConnectionDiagnostic(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connectionDiagnostic, nil
}
