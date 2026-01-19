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

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type appAuthConfigCollection struct {
	configs []*appauthconfigv1.AppAuthConfig
}

// NewAppAuthConfigCollection creates a [Collection] over the provided app auth
// configs.
func NewAppAuthConfigCollection(configs []*appauthconfigv1.AppAuthConfig) Collection {
	return &appAuthConfigCollection{configs}
}

func (c *appAuthConfigCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.configs))
	for _, resource := range c.configs {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *appAuthConfigCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Kind", "Match labels"}

	var rows [][]string
	for _, item := range c.configs {
		subKind := "undefined"
		switch item.Spec.SubKindSpec.(type) {
		case *appauthconfigv1.AppAuthConfigSpec_Jwt:
			subKind = "JWT"
		}

		rows = append(rows, []string{
			item.Metadata.Name,
			subKind,
			// Always format in verbose given that internal labels can be used
			// as matchers and should be shown to the users.
			common.FormatMultiValueLabels(label.ToMap(item.Spec.AppLabels), true),
		})
	}

	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func appAuthConfigHandler() Handler {
	return Handler{
		getHandler:    getAppAuthConfig,
		createHandler: createAppAuthConfig,
		updateHandler: updateAppAuthConfig,
		deleteHandler: deleteAppAuthConfig,
		singleton:     false,
		mfaRequired:   false,
		description:   "Application authentication configuration.",
	}
}

func getAppAuthConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	c := client.AppAuthConfigClient()
	if ref.Name != "" {
		config, err := c.GetAppAuthConfig(ctx, &appauthconfigv1.GetAppAuthConfigRequest{Name: ref.Name})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &appAuthConfigCollection{configs: []*appauthconfigv1.AppAuthConfig{config}}, nil
	}

	configs, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, pageToken string) ([]*appauthconfigv1.AppAuthConfig, string, error) {
		resp, err := c.ListAppAuthConfigs(ctx, &appauthconfigv1.ListAppAuthConfigsRequest{
			PageSize:  int32(limit),
			PageToken: pageToken,
		})

		return resp.GetConfigs(), resp.GetNextPageToken(), trace.Wrap(err)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &appAuthConfigCollection{configs}, nil
}

func createAppAuthConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	in, err := services.UnmarshalProtoResource[*appauthconfigv1.AppAuthConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.AppAuthConfigClient()
	if opts.Force {
		if _, err := c.UpsertAppAuthConfig(ctx, &appauthconfigv1.UpsertAppAuthConfigRequest{
			Config: in,
		}); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if _, err := c.CreateAppAuthConfig(ctx, &appauthconfigv1.CreateAppAuthConfigRequest{
			Config: in,
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Printf("App auth config %q has been created\n", in.GetMetadata().GetName())
	return nil
}

func updateAppAuthConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	in, err := services.UnmarshalProtoResource[*appauthconfigv1.AppAuthConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.AppAuthConfigClient()
	if _, err := c.UpdateAppAuthConfig(ctx, &appauthconfigv1.UpdateAppAuthConfigRequest{
		Config: in,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("App auth config %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func deleteAppAuthConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	c := client.AppAuthConfigClient()
	_, err := c.DeleteAppAuthConfig(ctx, &appauthconfigv1.DeleteAppAuthConfigRequest{
		Name: ref.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("App auth config %q has been deleted\n", ref.Name)
	return nil
}
