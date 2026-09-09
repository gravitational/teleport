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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type healthCheckConfigCollection struct {
	items []*healthcheckconfigv1.HealthCheckConfig
}

// NewHealthCheckConfigCollection creates a [Collection] over the provided
// health check configs.
func NewHealthCheckConfigCollection(items []*healthcheckconfigv1.HealthCheckConfig) Collection {
	return &healthCheckConfigCollection{items: items}
}

func (c *healthCheckConfigCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c.items))
	for _, item := range c.items {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c *healthCheckConfigCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Interval", "Timeout", "Healthy Threshold", "Unhealthy Threshold", "DB Labels", "DB Expression"}
	var rows [][]string
	for _, item := range c.items {
		meta := item.GetMetadata()
		spec := item.GetSpec()
		rows = append(rows, []string{
			meta.GetName(),
			common.FormatDefault(spec.GetInterval().AsDuration(), defaults.HealthCheckInterval),
			common.FormatDefault(spec.GetTimeout().AsDuration(), defaults.HealthCheckTimeout),
			common.FormatDefault(spec.GetHealthyThreshold(), defaults.HealthCheckHealthyThreshold),
			common.FormatDefault(spec.GetUnhealthyThreshold(), defaults.HealthCheckUnhealthyThreshold),
			common.FormatMultiValueLabels(label.ToMap(spec.GetMatch().GetDbLabels()), verbose),
			spec.GetMatch().GetDbLabelsExpression(),
		})
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "DB Labels")
	}

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func healthCheckConfigHandler() Handler {
	return Handler{
		getHandler:    getHealthCheckConfig,
		createHandler: createHealthCheckConfig,
		updateHandler: updateHealthCheckConfig,
		deleteHandler: deleteHealthCheckConfig,
		description:   "Configures how Teleport agents health check served resources.",
	}
}

func getHealthCheckConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		cfg, err := client.GetHealthCheckConfig(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &healthCheckConfigCollection{
			items: []*healthcheckconfigv1.HealthCheckConfig{cfg},
		}, nil
	}

	items, err := stream.Collect(clientutils.Resources(ctx, client.ListHealthCheckConfigs))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &healthCheckConfigCollection{items: items}, nil
}

func createHealthCheckConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	in, err := services.UnmarshalHealthCheckConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	createFn := client.CreateHealthCheckConfig
	if opts.Force {
		createFn = client.UpsertHealthCheckConfig
	}
	if _, err := createFn(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been created\n", in.GetMetadata().GetName())
	return nil
}

func updateHealthCheckConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	in, err := services.UnmarshalHealthCheckConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.UpdateHealthCheckConfig(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func deleteHealthCheckConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteHealthCheckConfig(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been deleted\n", ref.Name)
	return nil
}
