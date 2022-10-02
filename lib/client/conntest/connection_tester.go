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

package conntest

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
)

// TestConnectionRequest contains
// - the identification of the resource kind and resource name to test
// - additional paramenters which depend on the actual kind of resource to test
// As an example, for SSH Node it also includes the User/Principal that will be used to login.
type TestConnectionRequest struct {
	// ResourceKind describes the type of resource to test.
	ResourceKind string `json:"resource_kind"`
	// ResourceName is the identification of the resource's instance to test.
	ResourceName string `json:"resource_name"`

	// SSHPrincipal is the Linux username to use in a connection test.
	// Specific to SSHTester.
	SSHPrincipal string `json:"ssh_principal,omitempty"`

	// DialTimeout when trying to connect to the destination host
	DialTimeout time.Duration `json:"dial_timeout,omitempty"`
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *TestConnectionRequest) CheckAndSetDefaults() error {
	if r.ResourceKind == "" {
		return trace.BadParameter("missing required parameter ResourceKind")
	}

	if r.ResourceName == "" {
		return trace.BadParameter("missing required parameter ResourceName")
	}

	if r.DialTimeout <= 0 {
		r.DialTimeout = defaults.DefaultDialTimeout
	}

	return nil
}

/*
ConnectionTester is a mechanism to test resource access.
The result is a list of traces generated in multiple checkpoints.
If the connection fails, those traces will be of precious help to the end-user.
*/
type ConnectionTester interface {
	// TestConnection implementations should be as close to a real-world scenario as possible.
	//
	// They should create a ConnectionDiagnostic and pass its id in their certificate when trying to connect to the resource.
	// The agent/server/node should check for the id in the certificate and add traces to the ConnectionDiagnostic
	// according to whether it passed certain checkpoints.
	TestConnection(context.Context, TestConnectionRequest) (types.ConnectionDiagnostic, error)
}

// ConnectionTesterConfig contains all the required variables to build a connection test.
type ConnectionTesterConfig struct {
	// ResourceKind contains the resource type to test.
	// You should use the types.Kind<Resource> strings.
	ResourceKind string

	// UserClient is an auth client that has a User's identity.
	// This is the user that is running the SSH Connection Test.
	UserClient auth.ClientI

	// ProxyHostPort is the proxy to use in the `--proxy` format (host:webPort,sshPort)
	ProxyHostPort string

	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool
}

// ConnectionTesterForKind returns the proper Tester given a resource name.
// It returns trace.NotImplemented if the resource kind does not have a tester.
func ConnectionTesterForKind(cfg ConnectionTesterConfig) (ConnectionTester, error) {
	switch cfg.ResourceKind {
	case types.KindNode:
		tester, err := NewSSHConnectionTester(
			SSHConnectionTesterConfig{
				UserClient:        cfg.UserClient,
				ProxyHostPort:     cfg.ProxyHostPort,
				TLSRoutingEnabled: cfg.TLSRoutingEnabled,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return tester, nil
	}

	return nil, trace.NotImplemented("resource %q does not have a connection tester", cfg.ResourceKind)
}
