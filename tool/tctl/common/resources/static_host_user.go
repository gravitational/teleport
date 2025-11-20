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
	"slices"
	"strconv"
	"strings"

	"github.com/gravitational/trace"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type staticHostUserCollection struct {
	items []*userprovisioningpb.StaticHostUser
}

func NewStaticHostUserCollection(hostUsers []*userprovisioningpb.StaticHostUser) Collection {
	return &staticHostUserCollection{
		items: hostUsers,
	}
}

func (c *staticHostUserCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

func (c *staticHostUserCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {

		for _, matcher := range item.Spec.Matchers {
			labelMap := label.ToMap(matcher.NodeLabels)
			labelStringMap := make(map[string]string, len(labelMap))
			for k, vals := range labelMap {
				labelStringMap[k] = fmt.Sprintf("[%s]", printSortedStringSlice(vals))
			}
			var uid string
			if matcher.Uid != 0 {
				uid = strconv.Itoa(int(matcher.Uid))
			}
			var gid string
			if matcher.Gid != 0 {
				gid = strconv.Itoa(int(matcher.Gid))
			}
			rows = append(rows, []string{
				item.GetMetadata().Name,
				common.FormatLabels(labelStringMap, verbose),
				matcher.NodeLabelsExpression,
				printSortedStringSlice(matcher.Groups),
				uid,
				gid,
			})
		}
	}
	headers := []string{"Login", "Node Labels", "Node Expression", "Groups", "Uid", "Gid"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Node Expression")
	}
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func printSortedStringSlice(s []string) string {
	s = slices.Clone(s)
	slices.Sort(s)
	return strings.Join(s, ",")
}

func staticHostUserHandler() Handler {
	return Handler{
		getHandler:    getStaticHostUser,
		createHandler: createStaticHostUser,
		updateHandler: updateStaticHostUser,
		deleteHandler: deleteStaticHostUser,
		singleton:     false,
		mfaRequired:   false,
		description:   "Provisions local users on matching hosts.",
	}
}

func getStaticHostUser(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	hostUserClient := client.StaticHostUserClient()
	if ref.Name != "" {
		hostUser, err := hostUserClient.GetStaticHostUser(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &staticHostUserCollection{items: []*userprovisioningpb.StaticHostUser{hostUser}}, nil
	}

	resources, err := stream.Collect(clientutils.Resources(ctx, hostUserClient.ListStaticHostUsers))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &staticHostUserCollection{items: resources}, nil
}

func createStaticHostUser(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	hostUser, err := services.UnmarshalProtoResource[*userprovisioningpb.StaticHostUser](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	c := client.StaticHostUserClient()
	if opts.Force {
		if _, err := c.UpsertStaticHostUser(ctx, hostUser); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("static host user %q has been updated\n", hostUser.GetMetadata().Name)
	} else {
		if _, err := c.CreateStaticHostUser(ctx, hostUser); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("static host user %q has been created\n", hostUser.GetMetadata().Name)
	}

	return nil
}

func updateStaticHostUser(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	hostUser, err := services.UnmarshalProtoResource[*userprovisioningpb.StaticHostUser](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.StaticHostUserClient().UpdateStaticHostUser(ctx, hostUser); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("static host user %q has been updated\n", hostUser.GetMetadata().Name)
	return nil
}

func deleteStaticHostUser(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	if err := client.StaticHostUserClient().DeleteStaticHostUser(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("static host user %q has been deleted\n", ref.Name)
	return nil
}
