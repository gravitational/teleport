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

	"github.com/gravitational/trace"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// startKubeIntegrationWatchers starts kube watchers that use integration for the credentials. Currently only
// EKS watchers can do that and they behave differently from non-integration ones - we install agent on the
// discovered clusters, instead of just proxying them.
func (s *Server) startKubeIntegrationWatchers() error {
	if len(s.getKubeIntegrationFetchers()) == 0 && s.dynamicMatcherWatcher == nil {
		return nil
	}

	var mu sync.Mutex
	// enrollingClusters keeps track of clusters that are in the process of being enrolled, so they are
	// not yet among existing clusters, but we also should not try to enroll them again.
	enrollingClusters := map[string]bool{}

	clt := s.AccessPoint

	releaseChannels := automaticupgrades.Channels{}
	if err := releaseChannels.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	watcher, err := common.NewWatcher(s.ctx, common.WatcherConfig{
		FetchersFn: func() []common.Fetcher {
			kubeIntegrationFetchers := s.getKubeIntegrationFetchers()
			s.submitFetchersEvent(kubeIntegrationFetchers)
			return kubeIntegrationFetchers
		},
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
				type regionIntegrationMapKey struct {
					region      string
					integration string
				}
				clustersByRegionAndIntegration := map[regionIntegrationMapKey][]types.DiscoveredEKSCluster{}
				for _, c := range newClusters {
					mapKey := regionIntegrationMapKey{
						region:      c.GetAWSConfig().Region,
						integration: c.GetIntegration(),
					}
					clustersByRegionAndIntegration[mapKey] = append(clustersByRegionAndIntegration[mapKey], c)

				}

				for key, val := range clustersByRegionAndIntegration {
					key, val := key, val
					go s.enrollEKSClusters(key.region, key.integration, val, agentVersion, &mu, enrollingClusters)
				}

			case <-s.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *Server) enrollEKSClusters(region, integration string, clusters []types.DiscoveredEKSCluster, agentVersion string, mu *sync.Mutex, enrollingClusters map[string]bool) {
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

	for _, kubeAppDiscovery := range []bool{true, false} {
		for _, c := range batchedClusters[kubeAppDiscovery] {
			clusterNames = append(clusterNames, c.GetAWSConfig().Name)
		}
		if len(clusterNames) == 0 {
			continue
		}

		rsp, err := s.AccessPoint.EnrollEKSClusters(ctx, &integrationv1.EnrollEKSClustersRequest{
			Integration:        integration,
			Region:             region,
			EksClusterNames:    clusterNames,
			EnableAppDiscovery: kubeAppDiscovery,
			AgentVersion:       agentVersion,
		})
		if err != nil {
			s.Log.WithError(err).Errorf("failed to enroll EKS clusters %v", clusterNames)
			continue
		}

		for _, r := range rsp.Results {
			if r.Error != "" {
				if !strings.Contains(r.Error, "teleport-kube-agent is already installed on the cluster") {
					s.Log.WithError(err).Errorf("failed to enroll EKS cluster %q", r.EksClusterName)
				} else {
					s.Log.Debugf("EKS cluster %q already has installed kube agent", r.EksClusterName)
				}
			} else {
				s.Log.Infof("successfully enrolled EKS cluster %q", r.EksClusterName)
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

type IntegrationFetcher interface {
	// GetIntegration returns the integration name that is used for getting credentials of the fetcher.
	GetIntegration() string
}

func (s *Server) getKubeFetchers(integration bool) []common.Fetcher {
	var kubeFetchers []common.Fetcher

	filterIntegrationFetchers := func(fetcher common.Fetcher) bool {
		f, ok := fetcher.(IntegrationFetcher)
		if !ok {
			return false
		}

		return f.GetIntegration() != ""
	}

	filterNonIntegrationFetchers := func(fetcher common.Fetcher) bool {
		f, ok := fetcher.(IntegrationFetcher)
		if !ok {
			return true
		}

		return f.GetIntegration() == ""
	}

	filter := filterIntegrationFetchers
	if !integration {
		filter = filterNonIntegrationFetchers
	}

	s.muDynamicKubeFetchers.RLock()
	for _, fetcherSet := range s.dynamicKubeFetchers {
		for _, f := range fetcherSet {
			if filter(f) {
				kubeFetchers = append(kubeFetchers, f)
			}
		}
	}
	s.muDynamicKubeFetchers.RUnlock()

	for _, f := range s.kubeFetchers {
		if filter(f) {
			kubeFetchers = append(kubeFetchers, f)
		}
	}

	return kubeFetchers
}

func (s *Server) getKubeIntegrationFetchers() []common.Fetcher {
	return s.getKubeFetchers(true)
}

func (s *Server) getKubeNonIntegrationFetchers() []common.Fetcher {
	return s.getKubeFetchers(false)
}
