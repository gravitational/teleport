/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package k8s

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func OutputV2ServiceBuilder(cfg *OutputV2Config, opts ...OutputV2Option) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &OutputV2Service{
			botAuthClient:             deps.Client,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
			defaultCredentialLifetime: bot.DefaultCredentialLifetime,
			cfg:                       cfg,
			proxyPinger:               deps.ProxyPinger,
			reloadCh:                  deps.ReloadCh,
			executablePath:            autoupdate.StableExecutable,
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
		}
		for _, opt := range opts {
			opt.applyToV2Output(svc)
		}
		svc.log = deps.LoggerForService(svc)
		svc.statusReporter = deps.StatusRegistry.AddService(svc.String())
		return svc, nil
	}
}

// OutputV1Option is an option that can be provided to customize the service.
type OutputV2Option interface{ applyToV2Output(*OutputV2Service) }

func (opt DefaultCredentialLifetimeOption) applyToV2Output(o *OutputV2Service) {
	o.defaultCredentialLifetime = opt.lifetime
}

// OutputV2Service produces credentials which can be used to connect to a
// Kubernetes Cluster through teleport.
type OutputV2Service struct {
	// botAuthClient should be an auth client using the bots internal identity.
	// This will not have any roles impersonated and should only be used to
	// fetch CAs.
	botAuthClient             *apiclient.Client
	botIdentityReadyCh        <-chan struct{}
	defaultCredentialLifetime bot.CredentialLifetime
	cfg                       *OutputV2Config
	log                       *slog.Logger
	proxyPinger               connection.ProxyPinger
	statusReporter            readyz.Reporter
	reloadCh                  <-chan struct{}
	// executablePath is called to get the path to the tbot executable.
	// Usually this is os.Executable
	executablePath    func() (string, error)
	identityGenerator *identity.Generator
	clientBuilder     *client.Builder
}

func (s *OutputV2Service) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("kubernetes-v2-output (%s)", s.cfg.Destination.String()),
	)
}

func (s *OutputV2Service) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *OutputV2Service) Run(ctx context.Context) error {
	return trace.Wrap(internal.RunOnInterval(ctx, internal.RunOnIntervalConfig{
		Service:         s.String(),
		Name:            "output-renewal",
		F:               s.generate,
		Interval:        cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).RenewalInterval,
		RetryLimit:      internal.RenewalRetryLimit,
		Log:             s.log,
		ReloadCh:        s.reloadCh,
		IdentityReadyCh: s.botIdentityReadyCh,
		StatusReporter:  s.statusReporter,
	}))
}

func (s *OutputV2Service) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"OutputV2Service/generate",
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

	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)
	id, err := s.identityGenerator.GenerateFacade(ctx,
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	)
	if err != nil {
		return trace.Wrap(err, "generating identity")
	}

	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	impersonatedClient, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	matches, err := fetchAllMatchingKubeClusters(ctx, impersonatedClient, s.cfg.Selectors)
	if err != nil {
		return trace.Wrap(err)
	}

	var clusterNames []string
	for _, m := range matches {
		clusterNames = append(clusterNames, m.cluster.GetName())
	}
	defaultNamespaces := map[string]string{}
	for _, m := range matches {
		if m.selector.DefaultNamespace != "" {
			if _, ok := defaultNamespaces[m.cluster.GetName()]; ok {
				s.log.WarnContext(
					ctx,
					"Multiple selectors match the same cluster with different default namespaces configured, last definition will take priority",
					"cluster", m.cluster.GetName(),
				)
			}
			defaultNamespaces[m.cluster.GetName()] = m.selector.DefaultNamespace
		}
	}

	clusterNames = utils.Deduplicate(clusterNames)

	s.log.InfoContext(
		ctx,
		"Generated identity for Kubernetes access",
		"matched_cluster_count", len(clusterNames),
		"identity", id.Get(),
	)

	// Ping the proxy to resolve connection addresses.
	proxyPong, err := s.proxyPinger.Ping(ctx)
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

	keyRing, err := internal.NewClientKeyRing(id.Get(), hostCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	status := &kubernetesStatusV2{
		clusterAddr:            clusterAddr,
		tlsServerName:          tlsServerName,
		credentials:            keyRing,
		teleportClusterName:    proxyPong.ClusterName,
		kubernetesClusterNames: clusterNames,
		defaultNamespaces:      defaultNamespaces,
	}

	return trace.Wrap(s.render(ctx, status, id.Get(), hostCAs))
}

// kubernetesStatus holds teleport client information necessary to populate a
// kubeconfig.
type kubernetesStatusV2 struct {
	clusterAddr            string
	teleportClusterName    string
	tlsServerName          string
	credentials            *libclient.KeyRing
	kubernetesClusterNames []string
	// defaultNamespace is map of the cluster name to the default namespace
	// which should be used for that cluster.
	defaultNamespaces map[string]string
}

// queryKubeClustersByLabels fetches a list of Kubernetes clusters matching the
// given label selector.
func queryKubeClustersByLabels(ctx context.Context, clt apiclient.GetResourcesClient, labels map[string]string) ([]types.KubeCluster, error) {
	ctx, span := tracer.Start(ctx, "queryKubeClustersByLabels")
	defer span.End()

	servers, err := apiclient.GetAllResources[types.KubeServer](ctx, clt, &proto.ListResourcesRequest{
		Namespace:    defaults.Namespace,
		ResourceType: types.KindKubeServer,
		Labels:       labels,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var clusters []types.KubeCluster
	for _, server := range servers {
		clusters = append(clusters, server.GetCluster())
	}

	return clusters, nil
}

type selectorMatch struct {
	selector *KubernetesSelector
	cluster  types.KubeCluster
}

// fetchAllMatchingKubeClusters returns a list of all clusters matching the
// given selectors.
func fetchAllMatchingKubeClusters(ctx context.Context, clt apiclient.GetResourcesClient, selectors []*KubernetesSelector) ([]selectorMatch, error) {
	ctx, span := tracer.Start(ctx, "findAllMatchingKubeClusters")
	defer span.End()

	matches := []selectorMatch{}
	for _, selector := range selectors {
		if selector.Name != "" {
			cluster, err := getKubeCluster(ctx, clt, selector.Name)
			if err != nil {
				// Clusters explicitly named should be a hard fail.
				return nil, trace.Wrap(err, "unable to fetch cluster %q by name", selector.Name)
			}

			matches = append(matches, selectorMatch{
				selector: selector,
				cluster:  cluster,
			})
			continue
		}

		labeledClusters, err := queryKubeClustersByLabels(ctx, clt, selector.Labels)
		if err != nil {
			// TODO: should this be a hard error, or should we log it and
			// attempt to fetch all clusters? (Or should users be able to
			// select hard fail behavior with a config option?)
			// (Hard fail may be more relevant to named clusters.)
			// (There may be some value in a configurable hard fail if 0
			// clusters are returned.)
			return nil, trace.Wrap(err, "unable to fetch clusters with labels %v", selector.Labels)
		}
		for _, cluster := range labeledClusters {
			matches = append(matches, selectorMatch{
				selector: selector,
				cluster:  cluster,
			})
		}
	}

	return matches, nil
}

func (s *OutputV2Service) render(
	ctx context.Context,
	status *kubernetesStatusV2,
	routedIdentity *identity.Identity,
	hostCAs []types.CertAuthority,
) error {
	ctx, span := tracer.Start(
		ctx,
		"OutputV2Service/render",
	)
	defer span.End()

	if err := internal.WriteIdentityFile(ctx, s.log, status.credentials, s.cfg.Destination); err != nil {
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
	destinationDir, isDirectoryDest := s.cfg.Destination.(*destination.Directory)
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
		kubeCfg, err = s.generateKubeConfigV2WithoutPlugin(status)
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

		kubeCfg, err = s.generateKubeConfigV2WithPlugin(status, destinationDir.Path, executablePath)
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

	if err := s.cfg.Destination.Write(ctx, internal.HostCAPath, concatCACerts(hostCAs)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// encodePathComponent appropriate base64 encodes an input string for path-based
// routing use.
func encodePathComponent(input string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(input))
}

// generateKubeConfigWithPlugin creates a Kubernetes config object with the
// given cluster config, using the `tbot kube credentials` auth helper plugin to
// fetch refreshed certificate data at runtime.
func (o *OutputV2Service) generateKubeConfigV2WithPlugin(ks *kubernetesStatusV2, destPath string, executablePath string) (*clientcmdapi.Config, error) {
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

	// Configure the cluster.
	clusterCAs, err := ks.credentials.RootClusterCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cas := bytes.Join(clusterCAs, []byte("\n"))
	if len(cas) == 0 {
		return nil, trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}

	absDestPath, err := filepath.Abs(destPath)
	if err != nil {
		return nil, trace.Wrap(err, "determining absolute path for destination")
	}

	// Configure primary user/AuthInfo.
	execArgs := []string{"kube", "credentials",
		fmt.Sprintf("--destination-dir=%s", absDestPath),
	}
	config.AuthInfos[ks.teleportClusterName] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    executablePath,
			Args:       execArgs,
		},
	}

	for i, cluster := range ks.kubernetesClusterNames {
		contextName, err := kubeconfig.ContextNameFromTemplate(o.cfg.ContextNameTemplate, ks.teleportClusterName, cluster)
		if err != nil {
			return nil, trace.Wrap(err, "templating context name")
		}

		suffix := fmt.Sprintf("/v1/teleport/%s/%s", encodePathComponent(ks.teleportClusterName), encodePathComponent(cluster))
		config.Clusters[contextName] = &clientcmdapi.Cluster{
			Server:        ks.clusterAddr + suffix,
			TLSServerName: ks.tlsServerName,

			// TODO: consider using CertificateAuthority (path) here to avoid
			// duplication. This branch (with plugin) already requires a file
			// destination so we can assume the CA is available.
			CertificateAuthorityData: cas,
		}

		// Link the context to the main user.
		config.Contexts[contextName] = &clientcmdapi.Context{
			Cluster:  contextName,
			AuthInfo: ks.teleportClusterName,
		}
		if ns, ok := ks.defaultNamespaces[cluster]; ok {
			config.Contexts[contextName].Namespace = ns
		}

		// Always set the current context to the first-matched cluster. This
		// won't be perfectly consistent if the first selector uses labels, so
		// we may want to consider some way to flag an explicitly default
		// context.
		if i == 0 {
			config.CurrentContext = contextName
		}
	}

	return config, nil
}

func (o *OutputV2Service) generateKubeConfigV2WithoutPlugin(ks *kubernetesStatusV2) (*clientcmdapi.Config, error) {
	config := clientcmdapi.NewConfig()

	// Configure the cluster.
	clusterCAs, err := ks.credentials.RootClusterCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cas := bytes.Join(clusterCAs, []byte("\n"))
	if len(cas) == 0 {
		return nil, trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}

	// Create a global AuthInfo for this cluster.
	config.AuthInfos[ks.teleportClusterName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: ks.credentials.TLSCert,
		ClientKeyData:         ks.credentials.TLSPrivateKey.PrivateKeyPEM(),
	}

	for i, cluster := range ks.kubernetesClusterNames {
		contextName, err := kubeconfig.ContextNameFromTemplate(o.cfg.ContextNameTemplate, ks.teleportClusterName, cluster)
		if err != nil {
			return nil, trace.Wrap(err, "templating context name")
		}

		suffix := fmt.Sprintf("/v1/teleport/%s/%s", encodePathComponent(ks.teleportClusterName), encodePathComponent(cluster))
		config.Clusters[contextName] = &clientcmdapi.Cluster{
			Server:                   ks.clusterAddr + suffix,
			TLSServerName:            ks.tlsServerName,
			CertificateAuthorityData: cas,
		}

		// Link the context to the main AuthInfo/user.
		config.Contexts[contextName] = &clientcmdapi.Context{
			Cluster:  contextName,
			AuthInfo: ks.teleportClusterName,
		}
		if ns, ok := ks.defaultNamespaces[cluster]; ok {
			config.Contexts[contextName].Namespace = ns
		}

		if i == 0 {
			config.CurrentContext = contextName
		}
	}

	return config, nil
}

// concatCACerts borrow's identityfile's CA cert concat method.
func concatCACerts(cas []types.CertAuthority) []byte {
	trusted := authclient.AuthoritiesToTrustedCerts(cas)

	var caCerts []byte
	for _, ca := range trusted {
		for _, cert := range ca.TLSCertificates {
			caCerts = append(caCerts, cert...)
		}
	}

	return caCerts
}
