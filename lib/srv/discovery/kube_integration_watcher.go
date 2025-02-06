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
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
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

	pingResponse, err := s.AccessPoint.Ping(s.ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	proxyPublicAddr := pingResponse.GetProxyPublicAddr()

	var versionGetter version.Getter
	if proxyPublicAddr == "" {
		// If there are no proxy services running, we might fail to get the proxy URL and build a client.
		// In this case we "gracefully" fallback to our own version.
		// This is not supposed to happen outside of tests as the discovery service must join via a proxy.
		s.Log.WarnContext(s.ctx,
			"Failed to determine proxy public address, agents will install our own Teleport version instead of the one advertised by the proxy.",
			"version", teleport.Version)
		versionGetter = version.NewStaticGetter(teleport.Version, nil)
	} else {
		versionGetter, err = versionGetterForProxy(s.ctx, proxyPublicAddr)
		if err != nil {
			s.Log.WarnContext(s.ctx,
				"Failed to build a version client, falling back to Discovery service Teleport version.",
				"error", err,
				"version", teleport.Version)
			versionGetter = version.NewStaticGetter(teleport.Version, nil)
		}
	}

	watcher, err := common.NewWatcher(s.ctx, common.WatcherConfig{
		FetchersFn: func() []common.Fetcher {
			kubeIntegrationFetchers := s.getKubeIntegrationFetchers()
			s.submitFetchersEvent(kubeIntegrationFetchers)
			return kubeIntegrationFetchers
		},
		Logger:         s.Log.With("kind", types.KindKubernetesCluster),
		DiscoveryGroup: s.DiscoveryGroup,
		Interval:       s.PollInterval,
		Origin:         types.OriginCloud,
		TriggerFetchC:  s.newDiscoveryConfigChangedSub(),
		PreFetchHookFn: s.kubernetesIntegrationWatcherIterationStarted,
		Clock:          s.clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go watcher.Start()

	go func() {
		for {
			resourcesFoundByGroup := make(map[awsResourceGroup]int)
			resourcesEnrolledByGroup := make(map[awsResourceGroup]int)

			select {
			case resources := <-watcher.ResourcesC():
				if len(resources) == 0 {
					continue
				}

				existingServers, err := clt.GetKubernetesServers(s.ctx)
				if err != nil {
					s.Log.WarnContext(s.ctx, "Failed to get Kubernetes servers from cache", "error", err)
					continue
				}

				existingClusters, err := clt.GetKubernetesClusters(s.ctx)
				if err != nil {
					s.Log.WarnContext(s.ctx, "Failed to get Kubernetes clusters from cache", "error", err)
					continue
				}

				agentVersion, err := s.getKubeAgentVersion(versionGetter)
				if err != nil {
					s.Log.WarnContext(s.ctx, "Could not get agent version to enroll EKS clusters", "error", err)
					continue
				}

				var newClusters []types.DiscoveredEKSCluster
				mu.Lock()
				for _, r := range resources {
					newCluster, ok := r.(types.DiscoveredEKSCluster)
					if !ok {
						continue
					}

					resourceGroup := awsResourceGroupFromLabels(newCluster.GetStaticLabels())
					resourcesFoundByGroup[resourceGroup] += 1

					if enrollingClusters[newCluster.GetAWSConfig().Name] ||
						slices.ContainsFunc(existingServers, func(c types.KubeServer) bool { return c.GetName() == newCluster.GetName() }) ||
						slices.ContainsFunc(existingClusters, func(c types.KubeCluster) bool { return c.GetName() == newCluster.GetName() }) {

						resourcesEnrolledByGroup[resourceGroup] += 1
						continue
					}

					newClusters = append(newClusters, newCluster)
				}
				mu.Unlock()

				for group, count := range resourcesFoundByGroup {
					s.awsEKSResourcesStatus.incrementFound(group, count)
				}

				if len(newClusters) == 0 {
					break
				}

				// When enrolling EKS clusters, client for enrollment depends on the region and integration used.
				type regionIntegrationMapKey struct {
					region              string
					integration         string
					discoveryConfigName string
				}
				clustersByRegionAndIntegration := map[regionIntegrationMapKey][]types.DiscoveredEKSCluster{}
				for _, c := range newClusters {
					mapKey := regionIntegrationMapKey{
						region:              c.GetAWSConfig().Region,
						integration:         c.GetIntegration(),
						discoveryConfigName: c.GetStaticLabels()[types.TeleportInternalDiscoveryConfigName],
					}
					clustersByRegionAndIntegration[mapKey] = append(clustersByRegionAndIntegration[mapKey], c)

				}

				for key, val := range clustersByRegionAndIntegration {
					key, val := key, val
					go s.enrollEKSClusters(key.region, key.integration, key.discoveryConfigName, val, agentVersion, &mu, enrollingClusters)
				}

			case <-s.ctx.Done():
				return
			}

			for group, count := range resourcesEnrolledByGroup {
				s.awsEKSResourcesStatus.incrementEnrolled(group, count)
			}
		}
	}()
	return nil
}

func (s *Server) kubernetesIntegrationWatcherIterationStarted() {
	allFetchers := s.getKubeIntegrationFetchers()
	if len(allFetchers) == 0 {
		return
	}

	s.submitFetchersEvent(allFetchers)

	awsResultGroups := libslices.FilterMapUnique(
		allFetchers,
		func(f common.Fetcher) (awsResourceGroup, bool) {
			include := f.GetDiscoveryConfigName() != "" && f.IntegrationName() != ""
			resourceGroup := awsResourceGroup{
				discoveryConfigName: f.GetDiscoveryConfigName(),
				integration:         f.IntegrationName(),
			}
			return resourceGroup, include
		},
	)

	discoveryConfigs := libslices.FilterMapUnique(awsResultGroups, func(g awsResourceGroup) (s string, include bool) {
		return g.discoveryConfigName, true
	})
	s.updateDiscoveryConfigStatus(discoveryConfigs...)
	s.awsEKSResourcesStatus.reset()
	for _, g := range awsResultGroups {
		s.awsEKSResourcesStatus.iterationStarted(g)
	}

	s.awsEKSTasks.reset()
}

func (s *Server) enrollEKSClusters(region, integration, discoveryConfigName string, clusters []types.DiscoveredEKSCluster, agentVersion string, mu *sync.Mutex, enrollingClusters map[string]bool) {
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

		s.upsertTasksForAWSEKSFailedEnrollments()
	}()

	// We sort input clusters into two batches - one that has Kubernetes App Discovery
	// enabled, and another one that doesn't have it.
	var batchedClusters = map[bool][]types.DiscoveredEKSCluster{}
	for _, c := range clusters {
		batchedClusters[c.GetKubeAppDiscovery()] = append(batchedClusters[c.GetKubeAppDiscovery()], c)
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(len(clusters))*30*time.Second)
	defer cancel()

	for _, kubeAppDiscovery := range []bool{true, false} {
		clustersByName := make(map[string]types.DiscoveredEKSCluster)
		for _, c := range batchedClusters[kubeAppDiscovery] {
			clustersByName[c.GetAWSConfig().Name] = c
		}
		clusterNames := slices.Collect(maps.Keys(clustersByName))
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
			s.awsEKSResourcesStatus.incrementFailed(awsResourceGroup{
				discoveryConfigName: discoveryConfigName,
				integration:         integration,
			}, len(clusterNames))
			s.Log.ErrorContext(ctx, "Failed to enroll EKS clusters", "cluster_names", clusterNames, "error", err)
			continue
		}

		for _, r := range rsp.Results {
			if r.Error != "" {
				s.awsEKSResourcesStatus.incrementFailed(awsResourceGroup{
					discoveryConfigName: discoveryConfigName,
					integration:         integration,
				}, 1)
				if !strings.Contains(r.Error, "teleport-kube-agent is already installed on the cluster") {
					s.Log.ErrorContext(ctx, "Failed to enroll EKS cluster", "cluster_name", r.EksClusterName, "issue_type", r.IssueType, "error", r.Error)
				} else {
					s.Log.DebugContext(ctx, "EKS cluster already has installed kube agent", "cluster_name", r.EksClusterName)
				}

				cluster, ok := clustersByName[r.EksClusterName]
				if !ok {
					s.Log.WarnContext(ctx, "Received an EnrollEKSCluster result for a cluster which was not part of the requested clusters", "cluster_name", r.EksClusterName, "clusters_install_request", clusterNames)
					continue
				}
				s.awsEKSTasks.addFailedEnrollment(
					awsEKSTaskKey{
						integration:     integration,
						issueType:       r.IssueType,
						accountID:       cluster.GetAWSConfig().AccountID,
						region:          cluster.GetAWSConfig().Region,
						appAutoDiscover: kubeAppDiscovery,
					},
					&usertasksv1.DiscoverEKSCluster{
						DiscoveryConfig: discoveryConfigName,
						DiscoveryGroup:  s.DiscoveryGroup,
						SyncTime:        timestamppb.New(s.clock.Now()),
						Name:            cluster.GetAWSConfig().Name,
					},
				)
				s.upsertTasksForAWSEKSFailedEnrollments()
			} else {
				s.Log.InfoContext(ctx, "Successfully enrolled EKS cluster", "cluster_name", r.EksClusterName)
			}
		}
	}
}

func (s *Server) getKubeAgentVersion(versionGetter version.Getter) (string, error) {
	return kubeutils.GetKubeAgentVersion(s.ctx, s.AccessPoint, s.ClusterFeatures(), versionGetter)
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

func versionGetterForProxy(ctx context.Context, proxyPublicAddr string) (version.Getter, error) {
	proxyClt, err := webclient.NewReusableClient(&webclient.Config{
		Context:   ctx,
		ProxyAddr: proxyPublicAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to build proxy client")
	}

	baseURL := &url.URL{
		Scheme:  "https",
		Host:    proxyPublicAddr,
		RawPath: path.Join("/webapi/automaticupgrades/channel", automaticupgrades.DefaultChannelName),
	}
	if err != nil {
		return nil, trace.Wrap(err, "crafting the channel base URL (this is a bug)")
	}

	return version.FailoverGetter{
		// We try getting the version via the new webapi
		version.NewProxyVersionGetter(proxyClt),
		// If this is not implemented, we fallback to the release channels
		version.NewBasicHTTPVersionGetter(baseURL),
	}, nil
}
