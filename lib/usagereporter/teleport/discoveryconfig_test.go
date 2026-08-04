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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMatcherTypeMaps(t *testing.T) {
	for _, tt := range []struct {
		name      string
		prefix    string
		supported []string
		mapping   map[string]prehogv1a.DiscoveryMatcherType
	}{
		{"aws", "DISCOVERY_MATCHER_TYPE_AWS_", types.SupportedAWSMatchers, awsMatcherTypes},
		{"azure", "DISCOVERY_MATCHER_TYPE_AZURE_", types.SupportedAzureMatchers, azureMatcherTypes},
		{"gcp", "DISCOVERY_MATCHER_TYPE_GCP_", types.SupportedGCPMatchers, gcpMatcherTypes},
		{"kube", "DISCOVERY_MATCHER_TYPE_KUBE_", types.SupportedKubernetesMatchers, kubeMatcherTypes},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Len(t, tt.mapping, len(tt.supported))

			for _, matcher := range tt.supported {
				got, ok := tt.mapping[matcher]
				require.True(t, ok, "matcher %q has no matcher type", matcher)

				want := tt.prefix + strings.ToUpper(strings.ReplaceAll(matcher, "-", "_"))
				require.Equal(t, want, got.String(), "matcher %q maps to the wrong matcher type", matcher)
			}
		})
	}

	t.Run("every matcher type is produced", func(t *testing.T) {
		produced := make(map[prehogv1a.DiscoveryMatcherType]string)
		for _, mapping := range []map[string]prehogv1a.DiscoveryMatcherType{
			awsMatcherTypes, azureMatcherTypes, gcpMatcherTypes, kubeMatcherTypes,
		} {
			for matcher, matcherType := range mapping {
				other, ok := produced[matcherType]
				require.False(t, ok, "matchers %q and %q both map to %s", matcher, other, matcherType)
				produced[matcherType] = matcher
			}
		}

		for value, name := range prehogv1a.DiscoveryMatcherType_name {
			if prehogv1a.DiscoveryMatcherType(value) == prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_UNSPECIFIED {
				continue
			}
			require.Contains(t, produced, prehogv1a.DiscoveryMatcherType(value), "no matcher maps to %s", name)
		}
	})
}

func TestMatcherTypes(t *testing.T) {
	for _, tt := range []struct {
		name string
		spec discoveryconfig.Spec
		want []prehogv1a.DiscoveryMatcherType
	}{
		{
			name: "sorted and deduplicated",
			spec: discoveryconfig.Spec{
				AWS: []types.AWSMatcher{
					{Types: []string{types.AWSMatcherRDS, types.AWSMatcherEC2}},
					{Types: []string{types.AWSMatcherRDS}},
				},
			},
			want: []prehogv1a.DiscoveryMatcherType{
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_EC2,
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_RDS,
			},
		},
		{
			name: "every cloud",
			spec: discoveryconfig.Spec{
				AWS:   []types.AWSMatcher{{Types: []string{types.AWSMatcherEC2}}},
				Azure: []types.AzureMatcher{{Types: []string{types.AzureMatcherSQLServer, types.AzureMatcherVM}}},
				GCP:   []types.GCPMatcher{{Types: []string{types.GCPMatcherKubernetes}}},
				Kube:  []types.KubernetesMatcher{{Types: []string{types.KubernetesMatchersApp}}},
			},
			want: []prehogv1a.DiscoveryMatcherType{
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_EC2,
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AZURE_VM,
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AZURE_SQLSERVER,
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_GCP_GKE,
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_KUBE_APP,
			},
		},
		{
			name: "access graph syncs excluded",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS:   []*types.AccessGraphAWSSync{{Integration: "aws-sync"}},
					Azure: []*types.AccessGraphAzureSync{{Integration: "azure-sync"}},
				},
			},
		},
		{
			name: "unmapped matcher type skipped",
			spec: discoveryconfig.Spec{
				AWS: []types.AWSMatcher{
					{Types: []string{"from-a-newer-version", types.AWSMatcherEC2}},
				},
			},
			want: []prehogv1a.DiscoveryMatcherType{
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_EC2,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, matcherTypes(tt.spec))
		})
	}
}

func TestIntegrationNames(t *testing.T) {
	for _, tt := range []struct {
		name string
		spec discoveryconfig.Spec
		want []string
	}{
		{
			name: "no integrations",
			spec: discoveryconfig.Spec{
				AWS: []types.AWSMatcher{{Types: []string{types.AWSMatcherEC2}}},
			},
		},
		{
			name: "aws and azure matchers",
			spec: discoveryconfig.Spec{
				AWS:   []types.AWSMatcher{{Integration: "aws-matcher"}},
				Azure: []types.AzureMatcher{{Integration: "azure-matcher"}},
			},
			want: []string{"aws-matcher", "azure-matcher"},
		},
		{
			name: "sorted and deduplicated",
			spec: discoveryconfig.Spec{
				AWS: []types.AWSMatcher{
					{Integration: "zeta"},
					{Integration: "alpha"},
					{Integration: "zeta"},
				},
			},
			want: []string{"alpha", "zeta"},
		},
		{
			name: "access graph syncs excluded",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS:   []*types.AccessGraphAWSSync{{Integration: "aws-sync"}},
					Azure: []*types.AccessGraphAzureSync{{Integration: "azure-sync"}},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, integrationNames(tt.spec))
		})
	}
}

func TestAccessGraphIntegrationNames(t *testing.T) {
	for _, tt := range []struct {
		name string
		spec discoveryconfig.Spec
		want []string
	}{
		{
			name: "no access graph",
			spec: discoveryconfig.Spec{
				AWS: []types.AWSMatcher{{Integration: "aws-matcher"}},
			},
		},
		{
			name: "aws and azure syncs",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS:   []*types.AccessGraphAWSSync{{Integration: "aws-sync"}},
					Azure: []*types.AccessGraphAzureSync{{Integration: "azure-sync"}},
				},
			},
			want: []string{"aws-sync", "azure-sync"},
		},
		{
			name: "sync on ambient credentials",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{{Regions: []string{"us-east-1"}}},
				},
			},
		},
		{
			name: "sorted and deduplicated",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{Integration: "zeta"},
						{Integration: "alpha"},
						{Integration: "zeta"},
					},
				},
			},
			want: []string{"alpha", "zeta"},
		},
		{
			name: "matcher integrations excluded",
			spec: discoveryconfig.Spec{
				AWS:         []types.AWSMatcher{{Integration: "aws-matcher"}},
				AccessGraph: &types.AccessGraphSync{},
			},
		},
		{
			name: "nil entries",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS:   []*types.AccessGraphAWSSync{nil},
					Azure: []*types.AccessGraphAzureSync{nil},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, accessGraphIntegrationNames(tt.spec))
		})
	}
}

func TestAccessGraphSyncProviders(t *testing.T) {
	for _, tt := range []struct {
		name string
		spec discoveryconfig.Spec
		want []prehogv1a.CloudProvider
	}{
		{
			name: "no access graph",
			spec: discoveryconfig.Spec{
				AWS: []types.AWSMatcher{{Types: []string{types.AWSMatcherEC2}}},
			},
		},
		{
			name: "aws only",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{{Regions: []string{"us-east-1"}}},
				},
			},
			want: []prehogv1a.CloudProvider{prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS},
		},
		{
			name: "azure only",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					Azure: []*types.AccessGraphAzureSync{{SubscriptionID: "sub"}},
				},
			},
			want: []prehogv1a.CloudProvider{prehogv1a.CloudProvider_CLOUD_PROVIDER_AZURE},
		},
		{
			name: "both clouds",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS:   []*types.AccessGraphAWSSync{{Regions: []string{"us-east-1"}}},
					Azure: []*types.AccessGraphAzureSync{{SubscriptionID: "sub"}},
				},
			},
			want: []prehogv1a.CloudProvider{
				prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS,
				prehogv1a.CloudProvider_CLOUD_PROVIDER_AZURE,
			},
		},
		{
			name: "access graph with no syncs",
			spec: discoveryconfig.Spec{AccessGraph: &types.AccessGraphSync{}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, accessGraphSyncProviders(tt.spec))
		})
	}
}

func TestMatcherProviders(t *testing.T) {
	for _, tt := range []struct {
		name string
		spec discoveryconfig.Spec
		want []prehogv1a.CloudProvider
	}{
		{
			name: "no matchers",
			spec: discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{{Regions: []string{"us-east-1"}}},
				},
			},
		},
		{
			name: "aws only",
			spec: discoveryconfig.Spec{
				AWS: []types.AWSMatcher{{Types: []string{types.AWSMatcherEC2}}},
			},
			want: []prehogv1a.CloudProvider{prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS},
		},
		{
			name: "kubernetes only",
			spec: discoveryconfig.Spec{
				Kube: []types.KubernetesMatcher{{Types: []string{types.KubernetesMatchersApp}}},
			},
			want: []prehogv1a.CloudProvider{prehogv1a.CloudProvider_CLOUD_PROVIDER_KUBERNETES},
		},
		{
			name: "every provider",
			spec: discoveryconfig.Spec{
				AWS:   []types.AWSMatcher{{Types: []string{types.AWSMatcherEC2}}},
				Azure: []types.AzureMatcher{{Types: []string{types.AzureMatcherVM}}},
				GCP:   []types.GCPMatcher{{Types: []string{types.GCPMatcherKubernetes}}},
				Kube:  []types.KubernetesMatcher{{Types: []string{types.KubernetesMatchersApp}}},
			},
			want: []prehogv1a.CloudProvider{
				prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS,
				prehogv1a.CloudProvider_CLOUD_PROVIDER_AZURE,
				prehogv1a.CloudProvider_CLOUD_PROVIDER_GCP,
				prehogv1a.CloudProvider_CLOUD_PROVIDER_KUBERNETES,
			},
		},
		{
			name: "many matchers for one provider report it once",
			spec: discoveryconfig.Spec{
				AWS: []types.AWSMatcher{
					{Types: []string{types.AWSMatcherEC2}},
					{Types: []string{types.AWSMatcherRDS}},
				},
			},
			want: []prehogv1a.CloudProvider{prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, matcherProviders(tt.spec))
		})
	}
}

func TestClientKind(t *testing.T) {
	for _, tt := range []struct {
		name      string
		userAgent string
		want      prehogv1a.ClientKind
	}{
		{
			name: "no user agent",
			want: prehogv1a.ClientKind_CLIENT_KIND_UNKNOWN,
		},
		{
			name:      "unrecognized client",
			userAgent: "curl/8.7.1",
			want:      prehogv1a.ClientKind_CLIENT_KIND_UNKNOWN,
		},
		{
			name:      "component without a version",
			userAgent: teleport.ComponentTSH,
			want:      prehogv1a.ClientKind_CLIENT_KIND_UNKNOWN,
		},
		{
			name:      "recognized client",
			userAgent: teleport.ComponentTerraformProvider + "/19.0.0",
			want:      prehogv1a.ClientKind_CLIENT_KIND_TERRAFORM_PROVIDER,
		},
		{
			name:      "grpc-go appends its own version",
			userAgent: teleport.ComponentWeb + "/19.0.0 grpc-go/1.76.0",
			want:      prehogv1a.ClientKind_CLIENT_KIND_WEB_UI,
		},
		{
			name:      "tctl",
			userAgent: teleport.ComponentTCTL + "/19.0.0",
			want:      prehogv1a.ClientKind_CLIENT_KIND_TCTL,
		},
		{
			name:      "tsh",
			userAgent: teleport.ComponentTSH + "/19.0.0",
			want:      prehogv1a.ClientKind_CLIENT_KIND_TSH,
		},
		{
			name:      "tbot",
			userAgent: teleport.ComponentTBot + "/19.0.0",
			want:      prehogv1a.ClientKind_CLIENT_KIND_TBOT,
		},
		{
			name:      "kube operator",
			userAgent: teleport.ComponentKubeOperator + "/19.0.0",
			want:      prehogv1a.ClientKind_CLIENT_KIND_KUBE_OPERATOR,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.userAgent != "" {
				ctx = grpcmetadata.NewIncomingContext(ctx,
					grpcmetadata.Pairs("user-agent", tt.userAgent))
			}

			require.Equal(t, tt.want, clientKindFromContext(ctx))
		})
	}

	t.Run("every client kind is produced", func(t *testing.T) {
		produced := make(map[prehogv1a.ClientKind]string)
		for component, kind := range userAgentClientKinds {
			other, ok := produced[kind]
			require.False(t, ok, "components %q and %q both map to %s", component, other, kind)
			produced[kind] = component
		}

		for value, name := range prehogv1a.ClientKind_name {
			kind := prehogv1a.ClientKind(value)
			if kind == prehogv1a.ClientKind_CLIENT_KIND_UNSPECIFIED || kind == prehogv1a.ClientKind_CLIENT_KIND_UNKNOWN {
				continue
			}
			require.Contains(t, produced, kind, "no user agent maps to %s", name)
		}
	})
}

func TestNewDiscoveryConfigChangedEvent(t *testing.T) {
	dc, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name:   "dc-name",
			Labels: map[string]string{types.SetupAttemptIDLabel: "attempt-id"},
		},
		discoveryconfig.Spec{
			DiscoveryGroup: "dg",
			AWS: []types.AWSMatcher{{
				Types:       []string{types.AWSMatcherRDS},
				Integration: "aws-integration",
				Regions:     []string{"us-east-1"},
			}},
			AccessGraph: &types.AccessGraphSync{
				AWS: []*types.AccessGraphAWSSync{{
					Integration: "sync-integration",
					Regions:     []string{"us-east-1"},
				}},
			},
		},
	)
	require.NoError(t, err)

	ctx := grpcmetadata.NewIncomingContext(context.Background(),
		grpcmetadata.Pairs("user-agent", teleport.ComponentWeb+"/19.0.0"))

	require.Equal(t, &DiscoveryConfigChangedEvent{
		DiscoveryConfigName:         "dc-name",
		DiscoveryGroup:              "dg",
		IntegrationNames:            []string{"aws-integration"},
		AccessGraphIntegrationNames: []string{"sync-integration"},
		AccessGraphSyncProviders: []prehogv1a.CloudProvider{
			prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS,
		},
		SetupAttemptID: "attempt-id",
		Action:         prehogv1a.DiscoveryConfigChangeAction_DISCOVERY_CONFIG_CHANGE_ACTION_CREATE,
		MatcherTypes: []prehogv1a.DiscoveryMatcherType{
			prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_RDS,
		},
		MatcherProviders: []prehogv1a.CloudProvider{
			prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS,
		},
		ClientKind: prehogv1a.ClientKind_CLIENT_KIND_WEB_UI,
	}, NewDiscoveryConfigChangedEvent(ctx, dc,
		prehogv1a.DiscoveryConfigChangeAction_DISCOVERY_CONFIG_CHANGE_ACTION_CREATE))
}

func TestDiscoveryConfigChangedEventAnonymize(t *testing.T) {
	anonymizer, err := utils.NewHMACAnonymizer(utils.AnonymizationKeyString("anon-key-or-cluster-id"))
	require.NoError(t, err)

	t.Run("every field", func(t *testing.T) {
		event := &DiscoveryConfigChangedEvent{
			DiscoveryConfigName:         "dc-name",
			DiscoveryGroup:              "dg",
			IntegrationNames:            []string{"aws-integration", "azure-integration"},
			AccessGraphIntegrationNames: []string{"sync-integration"},
			AccessGraphSyncProviders: []prehogv1a.CloudProvider{
				prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS,
			},
			SetupAttemptID: "attempt-id",
			Action:         prehogv1a.DiscoveryConfigChangeAction_DISCOVERY_CONFIG_CHANGE_ACTION_UPSERT,
			MatcherTypes: []prehogv1a.DiscoveryMatcherType{
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_RDS,
			},
			MatcherProviders: []prehogv1a.CloudProvider{
				prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS,
			},
			ClientKind: prehogv1a.ClientKind_CLIENT_KIND_TCTL,
		}

		want := &prehogv1a.DiscoveryConfigChangedEvent{
			DiscoveryConfigId: anonymizer.AnonymizeString("dc-name"),
			DiscoveryGroupId:  anonymizer.AnonymizeString("dg"),
			IntegrationIds: []string{
				anonymizer.AnonymizeString("aws-integration"),
				anonymizer.AnonymizeString("azure-integration"),
			},
			AccessGraphIntegrationIds: []string{anonymizer.AnonymizeString("sync-integration")},
			AccessGraphSyncProviders: []prehogv1a.CloudProvider{
				prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS,
			},
			SetupAttemptId: anonymizer.AnonymizeString("attempt-id"),
			Action:         prehogv1a.DiscoveryConfigChangeAction_DISCOVERY_CONFIG_CHANGE_ACTION_UPSERT,
			MatcherTypes: []prehogv1a.DiscoveryMatcherType{
				prehogv1a.DiscoveryMatcherType_DISCOVERY_MATCHER_TYPE_AWS_RDS,
			},
			MatcherProviders: []prehogv1a.CloudProvider{
				prehogv1a.CloudProvider_CLOUD_PROVIDER_AWS,
			},
			ClientKind: prehogv1a.ClientKind_CLIENT_KIND_TCTL,
		}

		got := event.Anonymize(anonymizer).GetDiscoveryConfigChanged()
		require.Empty(t, cmp.Diff(want, got, protocmp.Transform()))
	})

	t.Run("no setup attempt", func(t *testing.T) {
		event := &DiscoveryConfigChangedEvent{DiscoveryConfigName: "dc-name"}

		require.Empty(t, event.Anonymize(anonymizer).GetDiscoveryConfigChanged().GetSetupAttemptId())
	})

	t.Run("no discovery group", func(t *testing.T) {
		event := &DiscoveryConfigChangedEvent{DiscoveryConfigName: "dc-name"}

		require.Empty(t, event.Anonymize(anonymizer).GetDiscoveryConfigChanged().GetDiscoveryGroupId())
	})
}
