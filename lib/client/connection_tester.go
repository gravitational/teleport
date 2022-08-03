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

/*
	ConnectionTester is a mechanism to test resource access
	The result is a list of traces generated in multiple checkpoints
	If the connection fails, those traces will be of precious help to the end-user
*/

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
)

// DiagnoseConnectionRequest contains
// - the identification of the resource kind and resource name to test
// - additional paramenters which depend on the actual kind of resource to test
// As an example, for SSH Node it also includes the User/Principal that will be used to login
type DiagnoseConnectionRequest struct {
	ResourceKind string `json:"resource_kind"`
	ResourceName string `json:"resource_name"`

	// Specific to SSHTester
	SSHPrincipal string `json:"ssh_principal,omitempty"`
}

// CheckAndSetDefaults validates the Request has the required fields
func (d *DiagnoseConnectionRequest) CheckAndSetDefaults() error {
	if d.ResourceKind == "" {
		return trace.BadParameter("missing required parameter Resource Kind")
	}

	if d.ResourceName == "" {
		return trace.BadParameter("missing required parameter Resource Name")
	}

	return nil
}

// ConnectionTester defines the interface for Connection Testers
type ConnectionTester interface {
	// TestConnection implementations should be as close to a real-world scenario as possible
	//
	// They should create a ConnectionDiagnostic and pass its id in their certificate when trying to connect to the resource
	// The agent/server/node should check for the id in the certificate and add traces to the ConnectionDiagnostic
	//   according to whether it passed certain checkpoints.
	TestConnection(context.Context, DiagnoseConnectionRequest) (types.ConnectionDiagnostic, error)
}

// ConnectionTesterForKind returns the proper Tester given a resource name
// It returns trace.NotImplemented if the resource kind does not have a tester
func ConnectionTesterForKind(resourceKind string, client auth.ClientI) (ConnectionTester, error) {
	switch resourceKind {
	case types.KindNode:
		return NewSSHConnectionTester(client), nil
	}
	return nil, trace.NotImplemented("resource '%s' does not have a connection tester", resourceKind)
}
