// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestDiscoveryServiceCollectionWriteText(t *testing.T) {
	t.Parallel()

	expires := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	collection := &discoveryServiceCollection{discoveryServices: []*discoveryservicev1.DiscoveryService{
		discoveryservicev1.DiscoveryService_builder{
			Metadata: headerv1.Metadata_builder{
				Name:    "host\x1b[31m-id",
				Expires: timestamppb.New(expires),
			}.Build(),
			Spec: discoveryservicev1.DiscoveryServiceSpec_builder{
				Hostname:        "host\nname",
				TeleportVersion: "18.0.0",
				DiscoveryGroup:  "prod\tgroup",
				PollInterval:    durationpb.New(5 * time.Minute),
				StaticMatcherCounts: map[string]int32{
					"aws": 2,
				},
				MatchersTruncated: true,
			}.Build(),
		}.Build(),
	}}

	var out bytes.Buffer
	require.NoError(t, collection.WriteText(&out, false))
	text := out.String()
	require.Contains(t, text, "18.0.0")
	require.Contains(t, text, "5m0s")
	require.Contains(t, text, expires.Format(time.RFC3339))
	require.Contains(t, text, "aws(2) (truncated)")
	require.NotContains(t, text, "\x1b[31m")
	require.False(t, strings.Contains(text, "host\nname"))
	require.False(t, strings.Contains(text, "prod\tgroup"))
}

func TestDiscoveryServiceCollectionStructuredOutputPreservesMatchers(t *testing.T) {
	t.Parallel()

	expires := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	collection := &discoveryServiceCollection{discoveryServices: []*discoveryservicev1.DiscoveryService{
		discoveryservicev1.DiscoveryService_builder{
			Metadata: headerv1.Metadata_builder{
				Name:    "host-id",
				Expires: timestamppb.New(expires),
			}.Build(),
			Spec: discoveryservicev1.DiscoveryServiceSpec_builder{
				PollInterval: durationpb.New(5 * time.Minute),
				StaticMatchers: discoveryservicev1.StaticMatchers_builder{
					Aws: []*types.AWSMatcher{{
						Types: []string{"ec2"},
						Tags:  types.Labels{"env": []string{"prod"}},
					}},
				}.Build(),
			}.Build(),
		}.Build(),
	}}

	encoded, err := json.Marshal(collection.Resources())
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"tags":{"env":"prod"}`,
		"structured output must preserve gogoproto custom matcher fields")
	require.Contains(t, string(encoded), `"expires":"2026-07-13T12:00:00Z"`)
	require.Contains(t, string(encoded), `"poll_interval":"300s"`)
	require.NotContains(t, string(encoded), `"seconds":`)
}

// TestStaticMatcherSummaryCoversEveryCountKey fails when a key is added to
// services.StaticMatcherCountKeyList without a matching branch in
// staticMatcherSummary, so a new matcher family cannot be silently omitted from
// the tctl "Static Matchers" column.
func TestStaticMatcherSummaryCoversEveryCountKey(t *testing.T) {
	t.Parallel()

	spec := discoveryservicev1.DiscoveryServiceSpec_builder{
		StaticMatchers: discoveryservicev1.StaticMatchers_builder{
			Aws:   []*types.AWSMatcher{{Types: []string{"ec2"}}},
			Azure: []*types.AzureMatcher{{Types: []string{"vm"}}},
			Gcp:   []*types.GCPMatcher{{Types: []string{"gce"}}},
			Kube:  []*types.KubernetesMatcher{{Types: []string{"app"}}},
			AccessGraph: &types.AccessGraphSync{
				AWS:   []*types.AccessGraphAWSSync{{Regions: []string{"us-east-1"}}},
				Azure: []*types.AccessGraphAzureSync{{SubscriptionID: "sub-1"}},
			},
		}.Build(),
	}.Build()

	summary := staticMatcherSummary(spec)
	for _, key := range services.StaticMatcherCountKeyList() {
		require.Contains(t, summary, key,
			"count key %q has no display branch in staticMatcherSummary (got %q)", key, summary)
	}
	// Every family must render a count, access_graph included: 1 AWS + 1
	// Azure sync matcher above must surface as access_graph(2), matching
	// the truncated path's access_graph(N) rendering.
	require.Equal(t, "aws(1),azure(1),gcp(1),kube(1),access_graph(2)", summary)
}

// TestStaticMatcherSummaryOmitsEmptyAccessGraph pins that a present but empty
// access_graph object renders nothing: it carries no sync matchers, so showing
// a bare access_graph marker would be indistinguishable from a configured one.
func TestStaticMatcherSummaryOmitsEmptyAccessGraph(t *testing.T) {
	t.Parallel()

	spec := discoveryservicev1.DiscoveryServiceSpec_builder{
		StaticMatchers: discoveryservicev1.StaticMatchers_builder{
			AccessGraph: &types.AccessGraphSync{},
		}.Build(),
	}.Build()

	require.Equal(t, "-", staticMatcherSummary(spec))
}
