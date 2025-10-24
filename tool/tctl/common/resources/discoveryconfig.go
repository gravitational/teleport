// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type discoveryConfigCollection struct {
	discoveryConfigs []*discoveryconfig.DiscoveryConfig
}

// NewDiscoveryConfigCollection creates a [Collection] over the provided discovery configs.
func NewDiscoveryConfigCollection(discoveryConfigs []*discoveryconfig.DiscoveryConfig) Collection {
	return &discoveryConfigCollection{discoveryConfigs: discoveryConfigs}
}

func (u *discoveryConfigCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(u.discoveryConfigs))
	for _, resource := range u.discoveryConfigs {
		r = append(r, resource)
	}
	return r
}

func (u *discoveryConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Discovery Group"})
	for _, discoveryConfig := range u.discoveryConfigs {
		t.AddRow([]string{
			discoveryConfig.GetName(),
			discoveryConfig.GetDiscoveryGroup(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func discoveryConfigHandler() Handler {
	return Handler{
		getHandler:    getDiscoveryConfig,
		createHandler: createDiscoveryConfig,
		updateHandler: updateDiscoveryConfig,
		deleteHandler: deleteDiscoveryConfig,
		singleton:     false,
		mfaRequired:   false,
		description:   "A configuration to auto discover and enroll resources in cloud providers.",
	}
}

// createDiscoveryConfig implements `tctl create discovery-config.yaml` command.
func createDiscoveryConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	discoveryConfig, err := services.UnmarshalDiscoveryConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	remote := client.DiscoveryConfigClient()

	if opts.Force {
		if _, err := remote.UpsertDiscoveryConfig(ctx, discoveryConfig); err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf("DiscoveryConfig %q has been written\n", discoveryConfig.GetName())
		return nil
	}

	if _, err := remote.CreateDiscoveryConfig(ctx, discoveryConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("DiscoveryConfig %q has been created\n", discoveryConfig.GetName())

	return nil
}

// getDiscoveryConfig implements `tctl get discovery_config/my-config` command.
func getDiscoveryConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	remote := client.DiscoveryConfigClient()
	if ref.Name != "" {
		dc, err := remote.GetDiscoveryConfig(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &discoveryConfigCollection{discoveryConfigs: []*discoveryconfig.DiscoveryConfig{dc}}, nil
	}

	resources, err := stream.Collect(clientutils.Resources(ctx, remote.ListDiscoveryConfigs))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &discoveryConfigCollection{discoveryConfigs: resources}, nil

}

// updateDiscoveryConfig implements `tctl create discovery-config.yaml` command.
func updateDiscoveryConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	discoveryConfig, err := services.UnmarshalDiscoveryConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	remote := client.DiscoveryConfigClient()
	if _, err := remote.UpdateDiscoveryConfig(ctx, discoveryConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("DiscoveryConfig %q has been updated\n", discoveryConfig.GetName())

	return nil
}

// deleteDiscoveryConfig implements `tctl rm discovery_config/my-config` command.
func deleteDiscoveryConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	remote := client.DiscoveryConfigClient()
	if err := remote.DeleteDiscoveryConfig(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("DiscoveryConfig %q has been deleted\n", ref.Name)
	return nil
}
