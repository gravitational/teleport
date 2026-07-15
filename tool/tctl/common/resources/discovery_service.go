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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type discoveryServiceCollection struct {
	discoveryServices []*discoveryservicev1.DiscoveryService
}

func (c *discoveryServiceCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.discoveryServices))
	for _, resource := range c.discoveryServices {
		r = append(r, discoveryServiceDisplayAdapter{
			Resource: types.ProtoResource153ToLegacy(resource),
			inner:    resource,
		})
	}
	return r
}

// discoveryServiceDisplayAdapter uses protojson for the resource envelope so
// timestamps and durations use their standard JSON representations, but
// static_matchers must never reach protojson: its embedded legacy gogoproto
// matcher types are unsupported by the protojson codec (a panic under current
// codegen), so they are cleared from a clone before envelope marshaling and
// restored from encoding/json, which keeps customtype fields such as matcher
// tags intact.
type discoveryServiceDisplayAdapter struct {
	types.Resource
	inner *discoveryservicev1.DiscoveryService
}

func (a discoveryServiceDisplayAdapter) MarshalJSON() ([]byte, error) {
	staticMatchers := a.inner.GetSpec().GetStaticMatchers()
	envelope := a.inner
	if staticMatchers != nil {
		envelope = proto.CloneOf(a.inner)
		envelope.GetSpec().SetStaticMatchers(nil)
	}
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(envelope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if staticMatchers == nil {
		return data, nil
	}

	var resource map[string]json.RawMessage
	if err := json.Unmarshal(data, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(resource["spec"], &spec); err != nil {
		return nil, trace.Wrap(err)
	}
	staticMatchersJSON, err := json.Marshal(staticMatchers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spec["static_matchers"] = staticMatchersJSON
	resource["spec"], err = json.Marshal(spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return json.Marshal(resource)
}

func (c *discoveryServiceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Hostname", "Version", "Group", "Poll Interval", "Static Matchers", "Expires"})
	for _, svc := range c.discoveryServices {
		spec := svc.GetSpec()
		pollInterval := "-"
		if spec.GetPollInterval() != nil {
			pollInterval = spec.GetPollInterval().AsDuration().String()
		}
		expires := "-"
		if svc.GetMetadata().GetExpires() != nil {
			expires = svc.GetMetadata().GetExpires().AsTime().Format(time.RFC3339)
		}
		t.AddRow([]string{
			utils.EscapeControl(svc.GetMetadata().GetName()),
			utils.EscapeControl(spec.GetHostname()),
			utils.EscapeControl(spec.GetTeleportVersion()),
			utils.EscapeControl(spec.GetDiscoveryGroup()),
			pollInterval,
			staticMatcherSummary(spec),
			expires,
		})
	}
	return trace.Wrap(t.WriteTo(w))
}

// staticMatcherSummary renders per-cloud matcher counts, e.g. "aws(2),gcp(1)".
// When the spec reports truncation, counts come from static_matcher_counts and
// are marked so partial visibility is never mistaken for the full picture.
func staticMatcherSummary(spec *discoveryservicev1.DiscoveryServiceSpec) string {
	if spec.GetMatchersTruncated() {
		// Render every key present rather than filtering to the known
		// constants: Auth enforced the vocabulary at admission, and a
		// second copy of it here could only drift. A side effect is that
		// a tctl one major behind the cluster (the supported client
		// skew) still renders keys it was not built with.
		counts := spec.GetStaticMatcherCounts()
		clouds := slices.Sorted(maps.Keys(counts))
		parts := make([]string, 0, len(clouds))
		for _, cloud := range clouds {
			parts = append(parts, fmt.Sprintf("%s(%d)", utils.EscapeControl(cloud), counts[cloud]))
		}
		if len(parts) == 0 {
			return "(truncated)"
		}
		return strings.Join(parts, ",") + " (truncated)"
	}
	m := spec.GetStaticMatchers()
	var parts []string
	if n := len(m.GetAws()); n > 0 {
		parts = append(parts, fmt.Sprintf("%s(%d)", services.StaticMatcherCountKeyAWS, n))
	}
	if n := len(m.GetAzure()); n > 0 {
		parts = append(parts, fmt.Sprintf("%s(%d)", services.StaticMatcherCountKeyAzure, n))
	}
	if n := len(m.GetGcp()); n > 0 {
		parts = append(parts, fmt.Sprintf("%s(%d)", services.StaticMatcherCountKeyGCP, n))
	}
	if n := len(m.GetKube()); n > 0 {
		parts = append(parts, fmt.Sprintf("%s(%d)", services.StaticMatcherCountKeyKube, n))
	}
	if ag := m.GetAccessGraph(); ag != nil {
		if n := len(ag.AWS) + len(ag.Azure); n > 0 {
			parts = append(parts, fmt.Sprintf("%s(%d)", services.StaticMatcherCountKeyAccessGraph, n))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

func discoveryServiceHandler() Handler {
	return Handler{
		getHandler:    getDiscoveryService,
		deleteHandler: deleteDiscoveryService,
		singleton:     false,
		mfaRequired:   false,
		description:   "The configuration heartbeat of a Discovery Service instance: its discovery group and effective static matchers",
	}
}

func getDiscoveryService(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		svc, err := client.GetDiscoveryService(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &discoveryServiceCollection{discoveryServices: []*discoveryservicev1.DiscoveryService{svc}}, nil
	}

	resources, err := stream.Collect(
		clientutils.Resources(ctx, client.ListDiscoveryServices),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &discoveryServiceCollection{discoveryServices: resources}, nil
}

func deleteDiscoveryService(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteDiscoveryService(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("discovery_service %+q has been deleted\n", ref.Name)
	return nil
}
