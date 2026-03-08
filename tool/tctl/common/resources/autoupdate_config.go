/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/trace"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func NewAutoUpdateConfigCollection(config *autoupdatev1pb.AutoUpdateConfig) Collection {
	return &autoUpdateConfigCollection{
		config: config,
	}
}

type autoUpdateConfigCollection struct {
	config *autoupdatev1pb.AutoUpdateConfig
}

func (c *autoUpdateConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.config)}
}

func (c *autoUpdateConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Tools AutoUpdate Enabled"})
	t.AddRow([]string{
		c.config.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.config.GetSpec().GetTools().GetMode()),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func autoUpdateConfigHandler() Handler {
	return Handler{
		getHandler:    getAutoUpdateConfig,
		createHandler: createAutoUpdateConfig,
		updateHandler: updateAutoUpdateConfig,
		deleteHandler: deleteAutoUpdateConfig,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures if, when, and how managed updates happen.",
	}
}

func getAutoUpdateConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	config, err := client.GetAutoUpdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &autoUpdateConfigCollection{config}, nil
}

func createAutoUpdateConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	rawJSON, err := normalizeAutoUpdateConfigDurations(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	config, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](rawJSON, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if config.GetMetadata() == nil {
		config.Metadata = &headerv1.Metadata{}
	}
	if config.GetMetadata().GetName() == "" {
		config.Metadata.Name = types.MetaNameAutoUpdateConfig
	}

	if opts.Force {
		_, err = client.UpsertAutoUpdateConfig(ctx, config)
	} else {
		_, err = client.CreateAutoUpdateConfig(ctx, config)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_config has been created")
	return nil
}

func updateAutoUpdateConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	rawJSON, err := normalizeAutoUpdateConfigDurations(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	config, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](rawJSON, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if config.GetMetadata() == nil {
		config.Metadata = &headerv1.Metadata{}
	}
	if config.GetMetadata().GetName() == "" {
		config.Metadata.Name = types.MetaNameAutoUpdateConfig
	}

	if _, err := client.UpdateAutoUpdateConfig(ctx, config); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("autoupdate_config has been updated")
	return nil
}

func deleteAutoUpdateConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteAutoUpdateConfig(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("AutoUpdateConfig has been deleted\n")
	return nil
}

// normalizeAutoUpdateConfigDurations rewrites human-friendly duration strings
// (e.g. "2h") into protobuf JSON duration strings (e.g. "7200s") for fields
// that are defined as google.protobuf.Duration.
//
// This is needed for spec.agents.maintenance_window_duration because users
// commonly provide Go-style values like "1h" as shown in the documentation,
// but protojson expects the "7200s" format.
func normalizeAutoUpdateConfigDurations(rawJSON []byte) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(rawJSON, &doc); err != nil {
		// If JSON doesn't parse, return as-is and let protojson report the error.
		return rawJSON, nil
	}

	spec, _ := doc["spec"].(map[string]any)
	agents, _ := spec["agents"].(map[string]any)
	if agents == nil {
		return rawJSON, nil
	}

	val, ok := agents["maintenance_window_duration"]
	if !ok {
		return rawJSON, nil
	}
	str, ok := val.(string)
	if !ok || str == "" {
		return rawJSON, nil
	}

	d, err := time.ParseDuration(str)
	if err != nil {
		// Not a valid Go duration; leave it for protojson to handle.
		return rawJSON, nil
	}

	seconds := d.Seconds()
	if seconds == float64(int64(seconds)) {
		agents["maintenance_window_duration"] = fmt.Sprintf("%ds", int64(seconds))
	} else {
		agents["maintenance_window_duration"] = fmt.Sprintf("%gs", seconds)
	}

	out, err := json.Marshal(doc)
	if err != nil {
		return rawJSON, nil
	}
	return out, nil
}
