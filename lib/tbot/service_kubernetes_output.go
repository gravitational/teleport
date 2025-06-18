/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"

	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const defaultKubeconfigPath = "kubeconfig.yaml"

// KubernetesOutputService produces credentials which can be used to connect to
// a Kubernetes Cluster through teleport.
type KubernetesOutputService struct {
	// botAuthClient should be an auth client using the bots internal identity.
	// This will not have any roles impersonated and should only be used to
	// fetch CAs.
	botAuthClient      *apiclient.Client
	botIdentityReadyCh <-chan struct{}
	botCfg             *config.BotConfig
	cfg                *config.KubernetesOutput
	getBotIdentity     getBotIdentityFn
	log                *slog.Logger
	proxyPingCache     *proxyPingCache
	reloadBroadcaster  *channelBroadcaster
	resolver           reversetunnelclient.Resolver
	// executablePath is called to get the path to the tbot executable.
	// Usually this is os.Executable
	executablePath func() (string, error)
}

func (s *KubernetesOutputService) String() string {
	return fmt.Sprintf("kubernetes-output (%s)", s.cfg.Destination.String())
}

func (s *KubernetesOutputService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *KubernetesOutputService) Run(ctx context.Context) error {
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	err := runOnInterval(ctx, runOnIntervalConfig{
		service:         s.String(),
		name:            "output-renewal",
		f:               s.generate,
		interval:        cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).RenewalInterval,
		retryLimit:      renewalRetryLimit,
		log:             s.log,
		reloadCh:        reloadCh,
		identityReadyCh: s.botIdentityReadyCh,
	})
	return trace.Wrap(err)
}

func (s *KubernetesOutputService) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"KubernetesOutputService/generate",
	)
	defer span.End()
	s.log.InfoContext(ctx, "Generating output")

	// Check the ACLs. We can't fix them, but we can warn if they're
	// misconfigured. We'll need to precompute a list of keys to check.
	// Note: This may only log a warning, depending on configuration.
	if err := s.cfg.Destination.Verify(identity.ListKeys(identity.DestinationKinds()...)); err != nil {
		return trace.Wrap(err)
	}
	// Ensure this destination is also writable. This is a hard fail if
	// ACLs are misconfigured, regardless of configuration.
	if err := identity.VerifyWrite(ctx, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "verifying destination")
	}

	var err error
	roles := s.cfg.Roles
	if len(roles) == 0 {
		roles, err = fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
		if err != nil {
			return trace.Wrap(err, "fetching default roles")
		}
	}

	id, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL,
		nil,
	)
	if err != nil {
		return trace.Wrap(err, "generating identity")
	}
	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	facade := identity.NewFacade(s.botCfg.FIPS, s.botCfg.Insecure, id)
	impersonatedClient, err := clientForFacade(ctx, s.log, s.botCfg, facade, s.resolver)
	if err != nil {
		return trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	kc, err := getKubeCluster(ctx, impersonatedClient, s.cfg.KubernetesCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	// make sure the output matches the fully resolved kube cluster name,
	// since it may have been just a "discovered name".
	kubeClusterName := kc.GetName()
	// Note: the Teleport server does attempt to verify k8s cluster names
	// and will fail to generate certs if the cluster doesn't exist or is
	// offline.

	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime)
	routedIdentity, err := generateIdentity(
		ctx,
		s.botAuthClient,
		id,
		roles,
		effectiveLifetime.TTL,
		func(req *proto.UserCertsRequest) {
			req.KubernetesCluster = kubeClusterName
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	warnOnEarlyExpiration(ctx, s.log.With("output", s), id, effectiveLifetime)

	s.log.InfoContext(
		ctx,
		"Generated identity for Kubernetes cluster",
		"kubernetes_cluster",
		kubeClusterName,
		"identity", describeTLSIdentity(ctx, s.log, routedIdentity),
	)

	// Ping the proxy to resolve connection addresses.
	proxyPong, err := s.proxyPingCache.ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	clusterAddr, tlsServerName, err := selectKubeConnectionMethod(proxyPong)
	if err != nil {
		return trace.Wrap(err)
	}

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO(noah): It's likely the Kubernetes output does not really need to
	// output these CAs - but - for backwards compat reasons, we output them.
	// Revisit this at a later date and make a call.
	userCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.UserCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	databaseCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.DatabaseCA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	keyRing, err := NewClientKeyRing(routedIdentity, hostCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	status := &kubernetesStatus{
		clusterAddr:           clusterAddr,
		tlsServerName:         tlsServerName,
		credentials:           keyRing,
		teleportClusterName:   proxyPong.ClusterName,
		kubernetesClusterName: kubeClusterName,
	}

	return s.render(ctx, status, routedIdentity, hostCAs, userCAs, databaseCAs)
}

func (s *KubernetesOutputService) render(
	ctx context.Context,
	status *kubernetesStatus,
	routedIdentity *identity.Identity,
	hostCAs, userCAs, databaseCAs []types.CertAuthority,
) error {
	ctx, span := tracer.Start(
		ctx,
		"KubernetesOutputService/render",
	)
	defer span.End()

	if err := writeIdentityFile(ctx, s.log, status.credentials, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "writing identity file")
	}
	if err := identity.SaveIdentity(
		ctx, routedIdentity, s.cfg.Destination, identity.DestinationKinds()...,
	); err != nil {
		return trace.Wrap(err, "persisting identity")
	}
	// In exec plugin mode, we write the credentials to disk and write a
	// kubeconfig that execs `tbot` to load those credentials.

	// We only support directory mode for this since the exec plugin needs
	// to know the path to read the credentials from, and this is
	// unpredictable with other types of destination.
	destinationDir, isDirectoryDest := s.cfg.Destination.(*config.DestinationDirectory)
	if !s.cfg.DisableExecPlugin {
		if !isDirectoryDest {
			slog.InfoContext(
				ctx,
				"Kubernetes template will be rendered without exec plugin because destination is not a directory. Explicitly set `disable_exec_plugin: true` in the output to suppress this message",
				"destination", logutils.StringerAttr(s.cfg.Destination))
			s.cfg.DisableExecPlugin = true
		}
	}

	var err error
	var kubeCfg *clientcmdapi.Config
	if s.cfg.DisableExecPlugin {
		// If they've disabled the exec plugin, we just write the credentials
		// directly into the kubeconfig.
		kubeCfg, err = generateKubeConfigWithoutPlugin(status)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		executablePath, err := s.executablePath()
		if errors.Is(err, autoupdate.ErrUnstableExecutable) {
			s.log.WarnContext(ctx, "Kubernetes template will be rendered with an unstable path to the tbot executable. Please reinstall tbot with Managed Updates to prevent instability.")
		} else if err != nil {
			return trace.Wrap(err)
		}

		kubeCfg, err = generateKubeConfigWithPlugin(status, destinationDir.Path, executablePath)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	yamlCfg, err := clientcmd.Write(*kubeCfg)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.cfg.Destination.Write(ctx, defaultKubeconfigPath, yamlCfg); err != nil {
		return trace.Wrap(err, "writing kubeconfig")
	}

	return trace.Wrap(writeTLSCAs(ctx, s.cfg.Destination, hostCAs, userCAs, databaseCAs))
}

// kubernetesStatus holds teleport client information necessary to populate a
// kubeconfig.
type kubernetesStatus struct {
	clusterAddr           string
	teleportClusterName   string
	kubernetesClusterName string
	tlsServerName         string
	credentials           *client.KeyRing
}

func generateKubeConfigWithoutPlugin(ks *kubernetesStatus) (*clientcmdapi.Config, error) {
	config := clientcmdapi.NewConfig()

	contextName := kubeconfig.ContextName(ks.teleportClusterName, ks.kubernetesClusterName)
	// Configure the cluster.
	clusterCAs, err := ks.credentials.RootClusterCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cas := bytes.Join(clusterCAs, []byte("\n"))
	if len(cas) == 0 {
		return nil, trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}
	config.Clusters[contextName] = &clientcmdapi.Cluster{
		Server:                   ks.clusterAddr,
		CertificateAuthorityData: cas,
		TLSServerName:            ks.tlsServerName,
	}

	config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: ks.credentials.TLSCert,
		ClientKeyData:         ks.credentials.TLSPrivateKey.PrivateKeyPEM(),
	}

	// Last, create a context linking the cluster to the auth info.
	config.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  contextName,
		AuthInfo: contextName,
	}
	config.CurrentContext = contextName

	return config, nil
}

// generateKubeConfigWithPlugin creates a Kubernetes config object with the given cluster
// config.
func generateKubeConfigWithPlugin(ks *kubernetesStatus, destPath string, executablePath string) (*clientcmdapi.Config, error) {
	config := clientcmdapi.NewConfig()

	// Implementation note: tsh/kube.go generates a kubeconfig with all
	// available clusters. This isn't especially useful in Machine ID when
	// there's _at most_ one cluster we have permission to access for this
	// Destination's set of certs, so instead of fetching all the k8s clusters
	// and adding everything, we'll just stick with the single cluster name in
	// our config file.
	// Otherwise, we adapt this from lib/kube/kubeconfig/kubeconfig.go's
	// Update(), but don't support env vars, insecure mode, or identity files.
	// We do still implement `tbot kube credentials` to help client apps
	// take better advantage of certificate renewals.

	contextName := kubeconfig.ContextName(ks.teleportClusterName, ks.kubernetesClusterName)

	// Configure the cluster.
	clusterCAs, err := ks.credentials.RootClusterCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cas := bytes.Join(clusterCAs, []byte("\n"))
	if len(cas) == 0 {
		return nil, trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}
	config.Clusters[contextName] = &clientcmdapi.Cluster{
		Server:                   ks.clusterAddr,
		CertificateAuthorityData: cas,
		TLSServerName:            ks.tlsServerName,
	}

	absDestPath, err := filepath.Abs(destPath)
	if err != nil {
		return nil, trace.Wrap(err, "determining absolute path for destination")
	}

	// Configure the auth info.
	execArgs := []string{"kube", "credentials",
		fmt.Sprintf("--destination-dir=%s", absDestPath),
	}
	config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    executablePath,
			Args:       execArgs,
		},
	}

	// Last, create a context linking the cluster to the auth info.
	config.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  contextName,
		AuthInfo: contextName,
	}
	config.CurrentContext = contextName

	return config, nil
}

// chooseOneKubeCluster chooses one matched kube cluster by name, or tries to
// choose one kube cluster by unambiguous "discovered name".
func chooseOneKubeCluster(clusters []types.KubeCluster, name string) (types.KubeCluster, error) {
	return chooseOneResource(clusters, name, "kubernetes cluster")
}

func getKubeCluster(ctx context.Context, clt apiclient.GetResourcesClient, name string) (types.KubeCluster, error) {
	ctx, span := tracer.Start(ctx, "getKubeCluster")
	defer span.End()

	servers, err := apiclient.GetAllResources[types.KubeServer](ctx, clt, &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindKubeServer,
		PredicateExpression: makeNameOrDiscoveredNamePredicate(name),
		Limit:               int32(defaults.DefaultChunkSize),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var clusters []types.KubeCluster
	for _, server := range servers {
		clusters = append(clusters, server.GetCluster())
	}

	clusters = types.DeduplicateKubeClusters(clusters)
	cluster, err := chooseOneKubeCluster(clusters, name)
	return cluster, trace.Wrap(err)
}

// selectKubeConnectionMethod determines the address and SNI that should be
// put into the kubeconfig file.
func selectKubeConnectionMethod(proxyPong *proxyPingResponse) (
	clusterAddr string, sni string, err error,
) {
	// First we check for TLS routing. If this is enabled, we use the Proxy's
	// PublicAddr, and we must also specify a special SNI.
	//
	// Even if KubePublicAddr is specified, we still use the general
	// PublicAddr when using TLS routing.
	if proxyPong.Proxy.TLSRoutingEnabled {
		addr, err := proxyPong.proxyWebAddr()
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return "", "", trace.Wrap(err, "parsing proxy public_addr")
		}

		return fmt.Sprintf("https://%s", addr), client.GetKubeTLSServerName(host), nil
	}

	// Next, we try to use the KubePublicAddr.
	if proxyPong.Proxy.Kube.PublicAddr != "" {
		return fmt.Sprintf("https://%s", proxyPong.Proxy.Kube.PublicAddr), "", nil
	}

	// Finally, we fall back to the main proxy PublicAddr with the port from
	// KubeListenAddr.
	if proxyPong.Proxy.Kube.ListenAddr != "" {
		host, _, err := net.SplitHostPort(proxyPong.Proxy.SSH.PublicAddr)
		if err != nil {
			return "", "", trace.Wrap(err, "parsing proxy public_addr")
		}

		_, port, err := net.SplitHostPort(proxyPong.Proxy.Kube.ListenAddr)
		if err != nil {
			return "", "", trace.Wrap(err, "parsing proxy kube_listen_addr")
		}

		return fmt.Sprintf("https://%s:%s", host, port), "", nil
	}

	return "", "", trace.BadParameter("unable to determine kubernetes address")
}
