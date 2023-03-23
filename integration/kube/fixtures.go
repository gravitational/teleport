// Copyright 2022 Gravitational, Inc
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

package kube

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// For this test suite to work, the target Kubernetes cluster must have the
// following RBAC objects configured:
// https://github.com/gravitational/teleport/blob/master/fixtures/ci-teleport-rbac/ci-teleport.yaml
const TestImpersonationGroup = "teleport-ci-test-group"

type ProxyConfig struct {
	T                   *helpers.TeleInstance
	Username            string
	PinnedIP            string
	KubeUsers           []string
	KubeGroups          []string
	Impersonation       *rest.ImpersonationConfig
	RouteToCluster      string
	CustomTLSServerName string
	TargetAddress       utils.NetAddr
}

// ProxyClient returns kubernetes client using local teleport proxy
func ProxyClient(cfg ProxyConfig) (*kubernetes.Clientset, *rest.Config, error) {
	ctx := context.Background()
	authServer := cfg.T.Process.GetAuthServer()
	clusterName, err := authServer.GetClusterName()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Fetch user info to get roles and max session TTL.
	user, err := authServer.GetUser(cfg.Username, false)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	roles, err := services.FetchRoles(user.GetRoles(), authServer, user.GetTraits())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ttl := roles.AdjustSessionTTL(10 * time.Minute)

	ca, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	caCert, signer, err := authServer.GetKeyStore().GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	privPEM, _, err := native.GenerateKeyPair()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	priv, err := tlsca.ParsePrivateKeyPEM(privPEM)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	id := tlsca.Identity{
		Username:         cfg.Username,
		Groups:           user.GetRoles(),
		KubernetesUsers:  cfg.KubeUsers,
		KubernetesGroups: cfg.KubeGroups,
		RouteToCluster:   cfg.RouteToCluster,
		PinnedIP:         cfg.PinnedIP,
	}
	subj, err := id.Subject()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     authServer.GetClock(),
		PublicKey: priv.Public(),
		Subject:   subj,
		NotAfter:  authServer.GetClock().Now().Add(ttl),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsClientConfig := rest.TLSClientConfig{
		CAData:     ca.GetActiveKeys().TLS[0].Cert,
		CertData:   cert,
		KeyData:    privPEM,
		ServerName: cfg.CustomTLSServerName,
	}
	config := &rest.Config{
		Host:            "https://" + cfg.T.Config.Proxy.Kube.ListenAddr.Addr,
		TLSClientConfig: tlsClientConfig,
	}
	if !cfg.TargetAddress.IsEmpty() {
		config.Host = "https://" + cfg.TargetAddress.Addr
	}
	if cfg.Impersonation != nil {
		config.Impersonate = *cfg.Impersonation
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return client, config, nil
}
