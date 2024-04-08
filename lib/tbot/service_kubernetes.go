package tbot

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

// KubernetesService is a temporary example service for testing purposes. It is
// not intended to be used and exists to demonstrate how a user configurable
// service integrates with the tbot service manager.
type KubernetesService struct {
	cfg *config.UnstableKubernetesService

	botCfg         *config.BotConfig
	proxyPingCache *proxyPingCache
	log            logrus.FieldLogger
	resolver       reversetunnelclient.Resolver
	botClient      *auth.Client
	getBotIdentity getBotIdentityFn
}

func (s *KubernetesService) OneShot(ctx context.Context) error {
	// Determine the roles to use for the impersonated k8s access user. We fall
	// back to all the roles the bot has if none are configured.
	roles := s.cfg.Roles
	var err error
	if len(roles) == 0 {
		roles, err = fetchDefaultRoles(ctx, s.botClient, s.getBotIdentity())
		if err != nil {
			return trace.Wrap(err)
		}
		s.log.WithField("roles", roles).Debug("No roles configured, using all roles available.")
	}

	impersonatedIdentity, err := generateIdentity(
		ctx,
		s.botClient,
		s.getBotIdentity(),
		roles,
		s.botCfg.CertificateTTL,
		nil,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	impersonatedClient, err := clientForFacade(
		ctx,
		s.log,
		s.botCfg,
		identity.NewFacade(s.botCfg.FIPS, s.botCfg.Insecure, impersonatedIdentity),
		s.resolver,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := impersonatedClient.Close(); err != nil {
			s.log.WithError(err).Error("Failed to close impersonated client.")
		}
	}()

	var clusters []types.KubeCluster
	if s.cfg.KubernetesCluster != "" {
		// Fetch one individual cluster
		cluster, err := getKubeCluster(ctx, impersonatedClient, s.cfg.KubernetesCluster)
		if err != nil {
			return trace.Wrap(err)

		}
		clusters = append(clusters, cluster)
	} else if len(s.cfg.KubernetesClusterLabels) > 0 {
		// Fetch by configured labels
		servers, err := apiclient.GetAllResources[types.KubeServer](ctx, impersonatedClient, &proto.ListResourcesRequest{
			Namespace:    defaults.Namespace,
			ResourceType: types.KindKubeServer,
			Labels:       s.cfg.KubernetesClusterLabels,
			Limit:        int32(defaults.DefaultChunkSize),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		for _, server := range servers {
			clusters = append(clusters, server.GetCluster())
		}
		clusters = types.DeduplicateKubeClusters(clusters)
	} else {
		return trace.BadParameter("kubernetes_cluster or kubernetes_cluster_labels must be set")
	}

	var clusterIdentities []struct {
		cluster  types.KubeCluster
		identity *identity.Identity
	}
	for _, cluster := range clusters {
		routedIdentity, err := generateIdentity(
			ctx,
			s.botClient,
			s.getBotIdentity(),
			roles,
			s.botCfg.CertificateTTL,
			func(req *proto.UserCertsRequest) {
				req.KubernetesCluster = cluster.GetName()
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
		clusterIdentities = append(clusterIdentities, struct {
			cluster  types.KubeCluster
			identity *identity.Identity
		}{
			cluster:  cluster,
			identity: routedIdentity,
		})
	}

	switch s.cfg.Format {
	case config.KubernetesServiceFormatKubeconfig:
		// Write all clusters to one kubeconfig:
	}

	return nil
}

func (s *KubernetesService) Run(ctx context.Context) error {
	return trace.NotImplemented("Run")
}

func (s *KubernetesService) String() string {
	return fmt.Sprintf("%s:%s", config.ServiceKubernetesType, s.cfg.Format)
}
