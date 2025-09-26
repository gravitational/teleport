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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

// ArgoCDServiceBuilder builds a new ArgoCDOutput.
func ArgoCDServiceBuilder(cfg *ArgoCDOutputConfig, opts ...ArgoCDServiceOption) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		svc := &ArgoCDOutput{
			cfg:                       cfg,
			defaultCredentialLifetime: bot.DefaultCredentialLifetime,
			proxyPinger:               deps.ProxyPinger,
			client:                    deps.Client,
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
			reloadCh:                  deps.ReloadCh,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
		}

		for _, opt := range opts {
			opt.applyToArgoOutput(svc)
		}

		// If no k8s client is provided, we attempt to create one from the
		// environment.
		if svc.k8s == nil {
			var err error
			if svc.k8s, err = newKubernetesClient(); err != nil {
				return nil, trace.Wrap(err, "creating Kubernetes client")
			}
		}

		svc.log = deps.LoggerForService(svc)
		svc.statusReporter = deps.StatusRegistry.AddService(svc.String())

		if svc.alpnUpgradeCache == nil {
			svc.alpnUpgradeCache = internal.NewALPNUpgradeCache(svc.log)
		}

		return svc, nil
	}
}

// ArgoCDServiceOption is an option that can be provided to customize the service.
type ArgoCDServiceOption interface{ applyToArgoOutput(*ArgoCDOutput) }

func (opt DefaultCredentialLifetimeOption) applyToArgoOutput(o *ArgoCDOutput) {
	o.defaultCredentialLifetime = opt.lifetime
}

func (opt KubernetesClientOption) applyToArgoOutput(o *ArgoCDOutput) { o.k8s = opt.client }
func (opt InsecureOption) applyToArgoOutput(o *ArgoCDOutput)         { o.insecure = opt.insecure }
func (opt ALPNUpgradeCacheOption) applyToArgoOutput(o *ArgoCDOutput) { o.alpnUpgradeCache = opt.cache }

// ArgoCDOutput registers Kubernetes clusters with ArgoCD by writing their
// details and the client certificate, etc. as Kubernetes secrets.
type ArgoCDOutput struct {
	cfg                       *ArgoCDOutputConfig
	defaultCredentialLifetime bot.CredentialLifetime
	k8s                       kubernetes.Interface

	log            *slog.Logger
	statusReporter readyz.Reporter
	reloadCh       <-chan struct{}

	identityGenerator *identity.Generator
	clientBuilder     *client.Builder
	proxyPinger       connection.ProxyPinger

	client             *apiclient.Client
	botIdentityReadyCh <-chan struct{}
	alpnUpgradeCache   *internal.ALPNUpgradeCache
	insecure           bool
}

// String returns the human-readable representation of the service that will be
// used in logs and the `/readyz` endpoints.
func (s *ArgoCDOutput) String() string {
	if s.cfg.Name != "" {
		return s.cfg.Name
	}
	var selectors []string
	for _, s := range s.cfg.Selectors {
		selectors = append(selectors, s.String())
	}
	return fmt.Sprintf("kubernetes-argo-cd-output (%s)", strings.Join(selectors, ", "))
}

// Run periodically refreshes the cluster credentials.
func (s *ArgoCDOutput) Run(ctx context.Context) error {
	err := internal.RunOnInterval(ctx, internal.RunOnIntervalConfig{
		Service: s.String(),
		Name:    "output-renewal",
		F: func(ctx context.Context) error {
			err := s.generate(ctx)

			// If the Teleport proxy is behind a TLS-terminating load balancer,
			// generate will return a NotImplemented error. We return nil here
			// because we do not want to RunOnInterval to retry.
			//
			// While we could have generate return nil in this case instead, we
			// do want to surface it as a hard error in one-shot mode.
			if trace.IsNotImplemented(err) {
				s.log.ErrorContext(ctx, "Failed to generate Argo CD cluster credentials", "error", err)
				return nil
			}

			return err
		},
		Interval:        cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).RenewalInterval,
		RetryLimit:      internal.RenewalRetryLimit,
		Log:             s.log,
		ReloadCh:        s.reloadCh,
		IdentityReadyCh: s.botIdentityReadyCh,
		StatusReporter:  s.statusReporter,
	})
	return trace.Wrap(err)
}

// OneShot generates cluster credentials once and exits.
func (s *ArgoCDOutput) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *ArgoCDOutput) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"ArgoCDOutput/generate",
	)
	defer span.End()

	clusters, err := s.discoverClusters(ctx)
	if err != nil {
		return trace.Wrap(err, "discovering clusters")
	}

	var errors []error
	for _, cluster := range clusters {
		secret, err := s.renderSecret(cluster)
		if err != nil {
			errors = append(errors, trace.Wrap(err, "rendering cluster secret"))
			continue
		}
		if err := s.writeSecret(ctx, secret); err != nil {
			errors = append(errors, trace.Wrap(err, "writing cluster secret"))
			continue
		}
	}
	return trace.NewAggregate(errors...)
}

func (s *ArgoCDOutput) discoverClusters(ctx context.Context) ([]*argoClusterCredentials, error) {
	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)
	id, err := s.identityGenerator.GenerateFacade(ctx,
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	)
	if err != nil {
		return nil, trace.Wrap(err, "generating identity")
	}

	impersonatedClient, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	matches, err := fetchAllMatchingKubeClusters(ctx, impersonatedClient, s.cfg.Selectors)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var clusterNames []string
	for _, m := range matches {
		clusterNames = append(clusterNames, m.cluster.GetName())
	}
	clusterNames = utils.Deduplicate(clusterNames)

	s.log.InfoContext(
		ctx,
		"Generated identity for Kubernetes access",
		"matched_cluster_count", len(clusterNames),
		"identity", id.Get(),
	)
	proxyPong, err := s.proxyPinger.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "pinging proxy to determine connection pathway")
	}

	proxyAddr, kubeSNI, err := selectKubeConnectionMethod(proxyPong)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if proxyPong.Proxy.TLSRoutingEnabled {
		required, err := s.alpnUpgradeCache.IsUpgradeRequired(ctx, proxyAddr, s.insecure)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if required {
			return nil, trace.NotImplemented(
				"Teleport proxy %q appears to be behind a TLS-terminating load balancer that does not support ALPN. "+
					"The %q service does not support this configuration, but you may be able to work around it by "+
					"running a local proxy with `tbot proxy kube` and configuring the cluster in Argo CD manually.",
				proxyAddr,
				ArgoCDOutputServiceType,
			)
		}
	}

	hostCAs, err := s.client.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyRing, err := internal.NewClientKeyRing(id.Get(), hostCAs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterCAs, err := keyRing.RootClusterCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caBytes := bytes.Join(clusterCAs, []byte("\n"))
	if len(caBytes) == 0 {
		return nil, trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}

	credentials := make([]*argoClusterCredentials, len(clusterNames))
	for idx, clusterName := range clusterNames {
		credentials[idx] = &argoClusterCredentials{
			teleportClusterName: proxyPong.ClusterName,
			kubeClusterName:     clusterName,
			addr: fmt.Sprintf(
				"%s/v1/teleport/%s/%s",
				proxyAddr,
				encodePathComponent(proxyPong.ClusterName),
				encodePathComponent(clusterName),
			),
			tlsClientConfig: argoTLSClientConfig{
				CAData:     caBytes,
				CertData:   keyRing.TLSCert,
				KeyData:    keyRing.TLSPrivateKey.PrivateKeyPEM(),
				ServerName: kubeSNI,
			},
			botName: id.Get().TLSIdentity.BotName,
		}
	}
	return credentials, nil
}

type argoClusterCredentials struct {
	teleportClusterName string
	kubeClusterName     string
	addr                string
	tlsClientConfig     argoTLSClientConfig
	botName             string
}

type argoTLSClientConfig struct {
	CAData     []byte `json:"caData"`
	CertData   []byte `json:"certData"`
	KeyData    []byte `json:"keyData"`
	ServerName string `json:"serverName,omitempty"`
}

func (s *ArgoCDOutput) renderSecret(cluster *argoClusterCredentials) (*corev1.Secret, error) {
	labels := map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
	}
	for k, v := range s.cfg.SecretLabels {
		// Do not overwrite any of "our" labels.
		if _, ok := labels[k]; !ok {
			labels[k] = v
		}
	}

	annotations := map[string]string{
		"teleport.dev/bot-name":                cluster.botName,
		"teleport.dev/kubernetes-cluster-name": cluster.kubeClusterName,
		"teleport.dev/updated":                 time.Now().Format(time.RFC3339),
		"teleport.dev/tbot-version":            teleport.Version,
		"teleport.dev/teleport-cluster-name":   cluster.teleportClusterName,
	}
	for k, v := range s.cfg.SecretAnnotations {
		// Do not overwrite any of "our" annotations.
		if _, ok := annotations[k]; !ok {
			annotations[k] = v
		}
	}

	configJSON, err := json.Marshal(struct {
		TLSClientConfig argoTLSClientConfig `json:"tlsClientConfig"`
	}{cluster.tlsClientConfig})
	if err != nil {
		return nil, trace.Wrap(err, "marshaling cluster credentials")
	}

	name, err := kubeconfig.ContextNameFromTemplate(
		s.cfg.ClusterNameTemplate,
		cluster.teleportClusterName,
		cluster.kubeClusterName,
	)
	if err != nil {
		return nil, trace.Wrap(err, "templating cluster name")
	}

	data := map[string][]byte{
		"name":   []byte(name),
		"server": []byte(cluster.addr),
		"config": configJSON,
	}

	if s.cfg.Project != "" {
		data["project"] = []byte(s.cfg.Project)
	}

	if len(s.cfg.Namespaces) != 0 {
		data["namespaces"] = []byte(strings.Join(s.cfg.Namespaces, ","))
		data["clusterResources"] = []byte(strconv.FormatBool(s.cfg.ClusterResources))
	}

	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: v1.ObjectMeta{
			Name:        s.secretName(cluster),
			Namespace:   s.cfg.SecretNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: data,
	}, nil
}

func (s *ArgoCDOutput) secretName(cluster *argoClusterCredentials) string {
	h := sha256.New()
	_, _ = h.Write([]byte(cluster.teleportClusterName))
	_, _ = h.Write([]byte(cluster.kubeClusterName))
	return fmt.Sprintf("%s.%x", s.cfg.SecretNamePrefix, h.Sum(nil)[:8])
}

func (s *ArgoCDOutput) writeSecret(ctx context.Context, secret *corev1.Secret) error {
	fullName := fmt.Sprintf("%s/%s", secret.GetNamespace(), secret.GetName())
	client := s.k8s.CoreV1().Secrets(secret.GetNamespace())

	existing, err := client.Get(ctx, secret.GetName(), v1.GetOptions{})
	if kubeerrors.IsNotFound(err) {
		// Secret is new, create it.
		if _, err := client.Create(ctx, secret, v1.CreateOptions{
			FieldManager: "tbot",
		}); err != nil {
			return trace.Wrap(err, "creating secret: %s", fullName)
		}
		return nil
	} else if err != nil {
		// Failed to read the secret.
		return trace.Wrap(err, "reading secret: %s", fullName)
	}

	// Secret exists, update it.
	secret.SetResourceVersion(secret.ResourceVersion)

	annotations := make(map[string]string)
	maps.Copy(annotations, existing.Annotations)
	maps.Copy(annotations, secret.Annotations)
	secret.SetAnnotations(annotations)

	// We use Update rather than Apply or Patch here because Argo CD will also
	// write to the secret (e.g. to add its own annotations or edit the config)
	// so it's likely we'd need to "force" apply our changes anyway.
	if _, err := client.Update(ctx, secret, v1.UpdateOptions{
		FieldManager: "tbot",
	}); err != nil {
		return trace.Wrap(err, "updating secret: %s", fullName)
	}
	return nil
}
