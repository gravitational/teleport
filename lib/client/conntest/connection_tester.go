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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
)

// TestConnectionRequest contains
// - the identification of the resource kind and resource name to test
// - additional paramenters which depend on the actual kind of resource to test
// As an example, for SSH Node it also includes the User/Principal that will be used to login.
type TestConnectionRequest struct {
	// MFAResponse is an optional field that holds a response to a MFA device challenge.
	MFAResponse client.MFAChallengeResponse `json:"mfa_response,omitempty"`
	// ResourceKind describes the type of resource to test.
	ResourceKind string `json:"resource_kind"`
	// ResourceName is the identification of the resource's instance to test.
	ResourceName string `json:"resource_name"`

	// DialTimeout when trying to connect to the destination host
	DialTimeout time.Duration `json:"dial_timeout,omitempty"`

	// InsecureSkipTLSVerify turns off verification for x509 upstream ALPN proxy service certificate.
	InsecureSkipVerify bool `json:"insecure_skip_verify,omitempty"`

	// SSHPrincipal is the Linux username to use in a connection test.
	// Specific to SSHTester.
	SSHPrincipal string `json:"ssh_principal,omitempty"`

	// KubernetesNamespace is the Kubernetes Namespace to List the Pods in.
	// Specific to KubernetesTester.
	KubernetesNamespace string `json:"kubernetes_namespace,omitempty"`

	// KubernetesImpersonation allows to configure a subset of `kubernetes_users` and
	// `kubernetes_groups` to impersonate.
	// Specific to KubernetesTester.
	KubernetesImpersonation KubernetesImpersonation `json:"kubernetes_impersonation,omitempty"`

	// DatabaseUser is the database User to be tested
	// Specific to DatabaseTester.
	DatabaseUser string `json:"database_user,omitempty"`

	// DatabaseName is the database user of the Database to be tested
	// Specific to DatabaseTester.
	DatabaseName string `json:"database_name,omitempty"`
}

// KubernetesImpersonation allows to configure a subset of `kubernetes_users` and
// `kubernetes_groups` to impersonate.
type KubernetesImpersonation struct {
	// KubernetesUser is the Kubernetes user to impersonate for this request.
	// Optional - If multiple values are configured the user must select one
	// otherwise the request will return an error.
	KubernetesUser string `json:"kubernetes_user,omitempty"`

	// KubernetesGroups are the Kubernetes groups to impersonate for this request.
	// Optional - If not specified it use all configured groups.
	// When KubernetesGroups is specified, KubernetesUser must be provided
	// as well.
	KubernetesGroups []string `json:"kubernetes_groups,omitempty"`
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *TestConnectionRequest) CheckAndSetDefaults() error {
	if r.ResourceKind == "" {
		return trace.BadParameter("missing required parameter ResourceKind")
	}

	if r.ResourceName == "" {
		return trace.BadParameter("missing required parameter ResourceName")
	}

	if r.KubernetesNamespace == "" {
		r.KubernetesNamespace = "default"
	}

	if r.DialTimeout <= 0 {
		r.DialTimeout = defaults.DefaultIOTimeout
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

	// PublicProxyAddr is public address of the proxy.
	PublicProxyAddr string

	// KubernetesPublicProxyAddr is the kubernetes proxy.
	KubernetesPublicProxyAddr string

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
		return tester, trace.Wrap(err)
	case types.KindKubernetesCluster:
		tester, err := NewKubeConnectionTester(
			KubeConnectionTesterConfig{
				UserClient:                cfg.UserClient,
				ProxyHostPort:             cfg.ProxyHostPort,
				TLSRoutingEnabled:         cfg.TLSRoutingEnabled,
				KubernetesPublicProxyAddr: cfg.KubernetesPublicProxyAddr,
			},
		)
		return tester, trace.Wrap(err)
	case types.KindDatabase:
		tester, err := NewDatabaseConnectionTester(
			DatabaseConnectionTesterConfig{
				UserClient:        cfg.UserClient,
				PublicProxyAddr:   cfg.PublicProxyAddr,
				TLSRoutingEnabled: cfg.TLSRoutingEnabled,
			},
		)
		return tester, trace.Wrap(err)
	default:
		return nil, trace.NotImplemented("resource %q does not have a connection tester", cfg.ResourceKind)
	}

}
