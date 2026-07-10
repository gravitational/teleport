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
	"strings"

	"github.com/gravitational/trace"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
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

// discoveryServiceDisplayAdapter overrides the display marshaling of the
// generic protoResource153 adapter, which uses protojson: protojson drops the
// gogoproto customtype fields (e.g. matcher tags) embedded in the spec.
// encoding/json on the hybrid-API struct renders them faithfully — the same
// reason the backend stores this resource with encoding/json.
type discoveryServiceDisplayAdapter struct {
	types.Resource
	inner *discoveryservicev1.DiscoveryService
}

func (a discoveryServiceDisplayAdapter) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.inner)
}

func (c *discoveryServiceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Hostname", "Group", "Static Matchers", "Bound Configs"})
	for _, svc := range c.discoveryServices {
		t.AddRow([]string{
			svc.GetMetadata().GetName(),
			svc.GetSpec().GetHostname(),
			svc.GetSpec().GetDiscoveryGroup(),
			staticMatcherSummary(svc.GetSpec().GetStaticMatchers()),
			bindingSummary(svc.GetSpec().GetBoundDiscoveryConfigs()),
		})
	}
	return trace.Wrap(t.WriteTo(w))
}

// staticMatcherSummary renders per-cloud matcher counts, e.g. "aws(2),gcp(1)".
func staticMatcherSummary(m *discoveryservicev1.StaticMatchers) string {
	var parts []string
	if n := len(m.GetAws()); n > 0 {
		parts = append(parts, fmt.Sprintf("aws(%d)", n))
	}
	if n := len(m.GetAzure()); n > 0 {
		parts = append(parts, fmt.Sprintf("azure(%d)", n))
	}
	if n := len(m.GetGcp()); n > 0 {
		parts = append(parts, fmt.Sprintf("gcp(%d)", n))
	}
	if n := len(m.GetKube()); n > 0 {
		parts = append(parts, fmt.Sprintf("kube(%d)", n))
	}
	if m.GetAccessGraph() != nil {
		parts = append(parts, "access_graph")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

// bindingSummary renders adopted config names, flagging failed adoptions,
// e.g. "rds-prod, eks-prod[ERROR]".
func bindingSummary(bindings []*discoveryservicev1.DiscoveryConfigBinding) string {
	if len(bindings) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		name := b.GetName()
		if b.GetState() != "LOADED" {
			name += "[" + b.GetState() + "]"
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}

func discoveryServiceHandler() Handler {
	return Handler{
		getHandler:    getDiscoveryService,
		deleteHandler: deleteDiscoveryService,
		singleton:     false,
		mfaRequired:   false,
		description:   "The configuration heartbeat of a Discovery Service instance: its discovery group, effective static matchers, and adopted discovery configs",
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
