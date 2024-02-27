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

package discovery

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func (s *Server) startKubeIntegrationWatchers() error {
	if s.dynamicMatcherWatcher == nil {
		return nil
	}

	var mu sync.Mutex
	// enrollingClusters keeps track of clusters that are in the process of being enrolled, so they are
	// not yet among existing clusters, but we also should not try to enroll them again.
	enrollingClusters := map[string]bool{}

	clt := s.AccessPoint

	pong, err := clt.Ping(s.ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	proxyPublicAddr := pong.GetProxyPublicAddr()

	releaseChannels := automaticupgrades.Channels{}
	if err := releaseChannels.CheckAndSetDefaults(s.ClusterFeatures()); err != nil {
		return trace.Wrap(err)
	}

	watcher, err := common.NewWatcher(s.ctx, common.WatcherConfig{
		FetchersFn:     s.getKubeIntegrationFetchers,
		Log:            s.Log.WithField("kind", types.KindKubernetesCluster),
		DiscoveryGroup: s.DiscoveryGroup,
		Interval:       s.PollInterval,
		Origin:         types.OriginCloud,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go watcher.Start()

	go func() {
		for {
			select {
			case resources := <-watcher.ResourcesC():
				if len(resources) == 0 {
					continue
				}

				existingServers, err := clt.GetKubernetesServers(s.ctx)
				if err != nil {
					s.Log.WithError(err).Warn("Failed to get Kubernetes servers from cache.")
					continue
				}

				existingClusters, err := clt.GetKubernetesClusters(s.ctx)
				if err != nil {
					s.Log.WithError(err).Warn("Failed to get Kubernetes clusters from cache.")
					continue
				}

				var newClusters []types.DiscoveredEKSCluster
				mu.Lock()
				for _, r := range resources {
					newCluster, ok := r.(types.DiscoveredEKSCluster)
					if !ok ||
						enrollingClusters[newCluster.GetAWSConfig().Name] ||
						slices.ContainsFunc(existingServers, func(c types.KubeServer) bool { return c.GetName() == newCluster.GetName() }) ||
						slices.ContainsFunc(existingClusters, func(c types.KubeCluster) bool { return c.GetName() == newCluster.GetName() }) {

						continue
					}

					newClusters = append(newClusters, newCluster)
				}
				mu.Unlock()

				if len(newClusters) == 0 {
					continue
				}

				agentVersion, err := s.getKubeAgentVersion(releaseChannels)
				if err != nil {
					s.Log.WithError(err).Warn("Could not get agent version to enroll EKS clusters")
					continue
				}

				// When enrolling EKS clusters, client for enrollment depends on the region and integration used.
				clustersByRegionAndIntegration := map[string]map[string][]types.DiscoveredEKSCluster{}
				for _, c := range newClusters {
					if _, ok := clustersByRegionAndIntegration[c.GetAWSConfig().Region]; !ok {
						clustersByRegionAndIntegration[c.GetAWSConfig().Region] = map[string][]types.DiscoveredEKSCluster{}
					}
					clustersByRegionAndIntegration[c.GetAWSConfig().Region][c.GetIntegration()] =
						append(clustersByRegionAndIntegration[c.GetAWSConfig().Region][c.GetIntegration()], c)
				}

				for region := range clustersByRegionAndIntegration {
					for integration := range clustersByRegionAndIntegration[region] {
						go s.enrollEKSClusters(region, integration, proxyPublicAddr, clustersByRegionAndIntegration[region][integration], agentVersion, &mu, enrollingClusters)
					}
				}

			case <-s.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *Server) enrollEKSClusters(region, integration, proxyPublicAddr string, clusters []types.DiscoveredEKSCluster, agentVersion string, mu *sync.Mutex, enrollingClusters map[string]bool) {
	enrollEKSClient, credsProvider, err := s.EKSEnrollmentClientGetter(s.ctx, integration, region)
	if err != nil {
		s.Log.WithError(err).Warn("Could not get EKS enrollment client")
		return
	}

	mu.Lock()
	for _, c := range clusters {
		if _, ok := enrollingClusters[c.GetAWSConfig().Name]; !ok {
			enrollingClusters[c.GetAWSConfig().Name] = true
		}
	}
	mu.Unlock()
	defer func() {
		// Clear enrolling clusters in the end.
		mu.Lock()
		for _, c := range clusters {
			delete(enrollingClusters, c.GetAWSConfig().Name)
		}
		mu.Unlock()
	}()

	// We sort input clusters into two batches - one that has Kubernetes App Discovery
	// enabled, and another one that doesn't have it.
	var batchedClusters = map[bool][]types.DiscoveredEKSCluster{}
	for _, c := range clusters {
		batchedClusters[c.GetKubeAppDiscovery()] = append(batchedClusters[c.GetKubeAppDiscovery()], c)
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(len(clusters))*30*time.Second)
	defer cancel()
	var clusterNames []string

	clusterFeatures := s.ClusterFeatures()

	for _, kubeAppDiscovery := range []bool{true, false} {
		for _, c := range batchedClusters[kubeAppDiscovery] {
			clusterNames = append(clusterNames, c.GetAWSConfig().Name)
		}
		if len(clusterNames) == 0 {
			continue
		}
		response := awsoidc.EnrollEKSClusters(ctx, s.Log, s.clock, proxyPublicAddr, credsProvider, enrollEKSClient, awsoidc.EnrollEKSClustersRequest{
			Region:             region,
			ClusterNames:       clusterNames,
			EnableAppDiscovery: kubeAppDiscovery,
			EnableAutoUpgrades: clusterFeatures.GetAutomaticUpgrades(),
			IsCloud:            clusterFeatures.GetCloud(),
			AgentVersion:       agentVersion,
		})

		for _, r := range response.Results {
			if r.Error != nil {
				if !trace.IsAlreadyExists(r.Error) {
					s.Log.WithError(err).Errorf("failed to enroll EKS cluster %q", r.ClusterName)
				} else {
					s.Log.Debugf("EKS cluster %q already has installed kube agent", r.ClusterName)
				}
			} else {
				s.Log.Infof("successfully enrolled EKS cluster %q", r.ClusterName)
			}
		}
	}
}

func (s *Server) getKubeAgentVersion(releaseChannels automaticupgrades.Channels) (string, error) {
	pingResponse, err := s.AccessPoint.Ping(s.ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	agentVersion := pingResponse.ServerVersion

	clusterFeatures := s.ClusterFeatures()
	if clusterFeatures.GetAutomaticUpgrades() {
		defaultVersion, err := releaseChannels.DefaultVersion(s.ctx)
		if err == nil {
			agentVersion = defaultVersion
		} else if !errors.Is(err, &version.NoNewVersionError{}) {
			return "", trace.Wrap(err)
		}
	}

	return strings.TrimPrefix(agentVersion, "v"), nil
}

func (s *Server) getEKSEnrollmentClient(ctx context.Context, integration, region string) (awsoidc.EnrollEKSCLusterClient, aws.CredentialsProvider, error) {
	awsClientReq, err := s.getAWSClientReq(ctx, integration, region)
	if err != nil {
		s.Log.WithError(err).Warn("Could not get AWS client request")
		return nil, nil, trace.Wrap(err)
	}

	enrollEKSClient, err := awsoidc.NewEnrollEKSClustersClient(ctx, awsClientReq, func(ctx context.Context, token types.ProvisionToken) error {
		return trace.NotImplemented("not implemented.")
	})
	if err != nil {
		s.Log.WithError(err).Warn("Could not get EKS enrollment client")
		return nil, nil, trace.Wrap(err)
	}

	credsProvider, err := awsoidc.NewAWSCredentialsProvider(ctx, awsClientReq)
	if err != nil {
		s.Log.WithError(err).Warn("Could not get AWS credentials provider")
		return nil, nil, trace.Wrap(err)
	}

	return enrollEKSClient, credsProvider, nil
}

func (s *Server) getAWSClientReq(ctx context.Context, integrationName, region string) (*awsoidc.AWSClientRequest, error) {
	clt := s.AccessPoint

	integration, err := clt.GetIntegration(ctx, integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if integration.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return nil, trace.BadParameter("integration subkind (%s) mismatch", integration.GetSubKind())
	}

	token, err := clt.GenerateAWSOIDCToken(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsoidcSpec := integration.GetAWSOIDCIntegrationSpec()
	if awsoidcSpec == nil {
		return nil, trace.BadParameter("missing spec fields for %q (%q) integration", integration.GetName(), integration.GetSubKind())
	}

	return &awsoidc.AWSClientRequest{
		IntegrationName: integrationName,
		Token:           token,
		RoleARN:         awsoidcSpec.RoleARN,
		Region:          region,
	}, nil
}

func (s *Server) getKubeIntegrationFetchers() []common.Fetcher {
	var kubeFetchers []common.Fetcher

	s.muDynamicKubeIntegrationFetchers.RLock()
	for _, fetcherSet := range s.dynamicKubeIntegrationFetchers {
		kubeFetchers = append(kubeFetchers, fetcherSet...)
	}
	s.muDynamicKubeIntegrationFetchers.RUnlock()

	s.submitFetchersEvent(kubeFetchers)

	return kubeFetchers
}
