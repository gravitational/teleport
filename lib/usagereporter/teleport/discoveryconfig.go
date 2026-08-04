/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package usagereporter

import (
	"context"
	"slices"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
)

var (
	awsMatcherTypes = map[string]prehogv1a.DiscoveryMatcherType{
		types.AWSMatcherEC2:                   prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_EC2,
		types.AWSMatcherEKS:                   prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_EKS,
		types.AWSMatcherRDS:                   prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_RDS,
		types.AWSMatcherRDSProxy:              prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_RDSPROXY,
		types.AWSMatcherRedshift:              prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_REDSHIFT,
		types.AWSMatcherRedshiftServerless:    prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_REDSHIFT_SERVERLESS,
		types.AWSMatcherElastiCache:           prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_ELASTICACHE,
		types.AWSMatcherElastiCacheServerless: prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_ELASTICACHE_SERVERLESS,
		types.AWSMatcherMemoryDB:              prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_MEMORYDB,
		types.AWSMatcherOpenSearch:            prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_OPENSEARCH,
		types.AWSMatcherDocumentDB:            prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_DOCDB,
	}
	azureMatcherTypes = map[string]prehogv1a.DiscoveryMatcherType{
		types.AzureMatcherVM:         prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AZURE_VM,
		types.AzureMatcherKubernetes: prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AZURE_AKS,
		types.AzureMatcherMySQL:      prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AZURE_MYSQL,
		types.AzureMatcherPostgres:   prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AZURE_POSTGRES,
		types.AzureMatcherRedis:      prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AZURE_REDIS,
		types.AzureMatcherSQLServer:  prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AZURE_SQLSERVER,
	}
	gcpMatcherTypes = map[string]prehogv1a.DiscoveryMatcherType{
		types.GCPMatcherKubernetes: prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_GCP_GKE,
		types.GCPMatcherCompute:    prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_GCP_GCE,
		types.GCPMatcherCloudSQL:   prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_GCP_CLOUDSQL,
	}
	kubeMatcherTypes = map[string]prehogv1a.DiscoveryMatcherType{
		types.KubernetesMatchersApp: prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_KUBE_APP,
	}
)

// matcherTypes returns the distinct resource types a config's matchers select,
// sorted.
func matcherTypes(spec discoveryconfig.Spec) []prehogv1a.DiscoveryMatcherType {
	var found []prehogv1a.DiscoveryMatcherType
	for _, m := range spec.AWS {
		for _, matcherType := range m.Types {
			if t, ok := awsMatcherTypes[matcherType]; ok {
				found = append(found, t)
			}
		}
	}
	for _, m := range spec.Azure {
		for _, matcherType := range m.Types {
			if t, ok := azureMatcherTypes[matcherType]; ok {
				found = append(found, t)
			}
		}
	}
	for _, m := range spec.GCP {
		for _, matcherType := range m.Types {
			if t, ok := gcpMatcherTypes[matcherType]; ok {
				found = append(found, t)
			}
		}
	}
	for _, m := range spec.Kube {
		for _, matcherType := range m.Types {
			if t, ok := kubeMatcherTypes[matcherType]; ok {
				found = append(found, t)
			}
		}
	}

	slices.Sort(found)
	return slices.Compact(found)
}

// integrationNames returns the distinct integrations a config's matchers use,
// sorted.
func integrationNames(spec discoveryconfig.Spec) []string {
	var found []string
	for _, m := range spec.AWS {
		if m.Integration != "" {
			found = append(found, m.Integration)
		}
	}
	for _, m := range spec.Azure {
		if m.Integration != "" {
			found = append(found, m.Integration)
		}
	}

	slices.Sort(found)
	return slices.Compact(found)
}

// accessGraphIntegrationNames returns the distinct integrations a config's
// Access Graph syncs use, sorted.
func accessGraphIntegrationNames(spec discoveryconfig.Spec) []string {
	if spec.AccessGraph == nil {
		return nil
	}

	var found []string
	for _, sync := range spec.AccessGraph.AWS {
		if sync != nil && sync.Integration != "" {
			found = append(found, sync.Integration)
		}
	}
	for _, sync := range spec.AccessGraph.Azure {
		if sync != nil && sync.Integration != "" {
			found = append(found, sync.Integration)
		}
	}

	slices.Sort(found)
	return slices.Compact(found)
}

// matcherProviders returns the platforms a config's matchers select resources
// from.
func matcherProviders(spec discoveryconfig.Spec) []prehogv1a.CloudProvider {
	var found []prehogv1a.CloudProvider
	if len(spec.AWS) > 0 {
		found = append(found, prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS)
	}
	if len(spec.Azure) > 0 {
		found = append(found, prehogv1a.CloudProvider_CLOUD_PROVIDER_AZURE)
	}
	if len(spec.GCP) > 0 {
		found = append(found, prehogv1a.CloudProvider_CLOUD_PROVIDER_GCP)
	}
	if len(spec.Kube) > 0 {
		found = append(found, prehogv1a.CloudProvider_CLOUD_PROVIDER_KUBERNETES)
	}

	return found
}

// accessGraphSyncProviders returns the clouds a config syncs into Access Graph.
func accessGraphSyncProviders(spec discoveryconfig.Spec) []prehogv1a.CloudProvider {
	if spec.AccessGraph == nil {
		return nil
	}

	var found []prehogv1a.CloudProvider
	if len(spec.AccessGraph.AWS) > 0 {
		found = append(found, prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS)
	}
	if len(spec.AccessGraph.Azure) > 0 {
		found = append(found, prehogv1a.CloudProvider_CLOUD_PROVIDER_AZURE)
	}

	return found
}

var userAgentClientKinds = map[string]prehogv1a.ClientKind{
	teleport.ComponentWeb:               prehogv1a.ClientKind_CLIENT_KIND_WEB_UI,
	teleport.ComponentTCTL:              prehogv1a.ClientKind_CLIENT_KIND_TCTL,
	teleport.ComponentTSH:               prehogv1a.ClientKind_CLIENT_KIND_TSH,
	teleport.ComponentTBot:              prehogv1a.ClientKind_CLIENT_KIND_TBOT,
	teleport.ComponentTerraformProvider: prehogv1a.ClientKind_CLIENT_KIND_TERRAFORM_PROVIDER,
	teleport.ComponentKubeOperator:      prehogv1a.ClientKind_CLIENT_KIND_KUBE_OPERATOR,
}

// clientKindFromContext returns the tool that made the request.
func clientKindFromContext(ctx context.Context) prehogv1a.ClientKind {
	userAgent := metadata.UserAgentFromContext(ctx)
	for component, kind := range userAgentClientKinds {
		if strings.HasPrefix(userAgent, component+"/") {
			return kind
		}
	}
	return prehogv1a.ClientKind_CLIENT_KIND_UNKNOWN
}

// NewDiscoveryConfigChangedEvent builds the event for one change to one
// discovery config.
func NewDiscoveryConfigChangedEvent(ctx context.Context, dc *discoveryconfig.DiscoveryConfig, action prehogv1a.DiscoveryConfigChangeAction) *DiscoveryConfigChangedEvent {
	return &DiscoveryConfigChangedEvent{
		DiscoveryConfigName:         dc.GetName(),
		DiscoveryGroup:              dc.Spec.DiscoveryGroup,
		IntegrationNames:            integrationNames(dc.Spec),
		AccessGraphIntegrationNames: accessGraphIntegrationNames(dc.Spec),
		AccessGraphSyncProviders:    accessGraphSyncProviders(dc.Spec),
		SetupAttemptID:              dc.GetMetadata().Labels[types.SetupAttemptIDLabel],
		Action:                      action,
		MatcherTypes:                matcherTypes(dc.Spec),
		MatcherProviders:            matcherProviders(dc.Spec),
		ClientKind:                  clientKindFromContext(ctx),
	}
}
