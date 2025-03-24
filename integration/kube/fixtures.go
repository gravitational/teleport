/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package kube

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/cryptosuites"
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
	KubeCluster         string
	Impersonation       *rest.ImpersonationConfig
	RouteToCluster      string
	CustomTLSServerName string
	TargetAddress       utils.NetAddr
}

// ProxyClient returns kubernetes client using local teleport proxy
func ProxyClient(cfg ProxyConfig) (*kubernetes.Clientset, *rest.Config, error) {
	ctx := context.Background()
	authServer := cfg.T.Process.GetAuthServer()
	clusterName, err := authServer.GetClusterName(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Fetch user info to get roles and max session TTL.
	user, err := authServer.GetUser(ctx, cfg.Username, false)
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
	priv, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	privPEM, err := keys.MarshalPrivateKey(priv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	kubeServers, _ := authServer.GetKubernetesServers(ctx)
	kubeCluster := cfg.KubeCluster
	if cfg.KubeCluster == "" && len(kubeServers) > 0 {
		kubeCluster = kubeServers[0].GetCluster().GetName()
	}

	id := tlsca.Identity{
		Username:          cfg.Username,
		Groups:            user.GetRoles(),
		KubernetesUsers:   cfg.KubeUsers,
		KubernetesGroups:  cfg.KubeGroups,
		RouteToCluster:    cfg.RouteToCluster,
		KubernetesCluster: kubeCluster,
		PinnedIP:          cfg.PinnedIP,
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
