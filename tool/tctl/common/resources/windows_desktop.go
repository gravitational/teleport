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
	"fmt"
	"io"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type windowsDesktopCollection struct {
	desktops []types.WindowsDesktop
}

func windowsDesktopHandler() Handler {
	return Handler{
		getHandler:    getDesktops,
		createHandler: createWindowsDesktop,
		deleteHandler: deleteWindowsDesktop,
		singleton:     false,
		mfaRequired:   false,
		description:   "A Windows remote desktop protected by Teleport",
	}
}

func (c *windowsDesktopCollection) Resources() (r []types.Resource) {
	for _, resource := range c.desktops {
		r = append(r, resource)
	}
	return r
}

func (c *windowsDesktopCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, d := range c.desktops {
		labels := common.FormatLabels(d.GetAllLabels(), verbose)
		rows = append(rows, []string{d.GetName(), d.GetAddr(), d.GetDomain(), d.GetHostID(), labels})
	}
	headers := []string{"Name", "Address", "AD Domain", "Host ID", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func getDesktops(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	// TODO(tross): DELETE IN v21.0.0, replace with regular Collect
	desktops, err := clientutils.CollectWithFallback(
		ctx,
		func(ctx context.Context, limit int, token string) ([]types.WindowsDesktop, string, error) {
			resp, err := client.ListWindowsDesktops(ctx,
				types.ListWindowsDesktopsRequest{
					WindowsDesktopFilter: types.WindowsDesktopFilter{Name: ref.Name},
					StartKey:             token,
					Limit:                limit,
				})
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			return resp.Desktops, resp.NextKey, nil
		},
		func(ctx context.Context) ([]types.WindowsDesktop, error) {
			return client.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{Name: ref.Name})
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &windowsDesktopCollection{desktops: desktops}, nil
}

func deleteWindowsDesktop(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	var hostIDs []string

	for desktop, err := range clientutils.Resources(ctx, func(ctx context.Context, size int, token string) ([]types.WindowsDesktop, string, error) {
		resp, err := client.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
			Limit:    size,
			StartKey: token,
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return resp.Desktops, resp.NextKey, nil
	}) {
		if err != nil {
			return trace.Wrap(err)
		}
		if desktop.GetName() == ref.Name {
			hostIDs = append(hostIDs, desktop.GetHostID())
		}
	}

	if len(hostIDs) == 0 {
		return trace.NotFound("no desktops with name %q were found", ref.Name)
	}

	deleted := 0
	var errs []error
	for _, hostID := range hostIDs {
		if err := client.DeleteWindowsDesktop(ctx, hostID, ref.Name); err != nil {
			errs = append(errs, err)
			continue
		}
		deleted++
	}

	fmts := "%d windows desktops with name %q have been deleted"
	if err := trace.NewAggregate(errs...); err != nil {
		fmt.Printf(fmts+" with errors while deleting\n", deleted, ref.Name)
		return err
	}
	fmt.Printf(fmts+"\n", deleted, ref.Name)
	return nil
}

func createWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	wd, err := services.UnmarshalWindowsDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.UpsertWindowsDesktop(ctx, wd); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("windows desktop %q has been created\n", wd.GetName())
	return nil
}

type windowsDesktopServiceCollection struct {
	services []types.WindowsDesktopService
}

func windowsDesktopServiceHandler() Handler {
	return Handler{
		getHandler:    getWindowsDesktopService,
		deleteHandler: deleteWindowsDesktopService,
		singleton:     false,
		mfaRequired:   false,
		description:   "A Teleport agent that proxies connections to Windows desktops",
	}
}

func (c *windowsDesktopServiceCollection) Resources() (r []types.Resource) {
	for _, resource := range c.services {
		r = append(r, resource)
	}
	return r
}

func (c *windowsDesktopServiceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Address", "Version"})
	for _, service := range c.services {
		addr := service.GetAddr()
		if addr == reversetunnelclient.LocalWindowsDesktop {
			addr = "<proxy tunnel>"
		}
		t.AddRow([]string{service.GetName(), addr, service.GetTeleportVersion()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func getWindowsDesktopService(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		service, err := client.GetWindowsDesktopService(ctx, ref.Name)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("Windows desktop service %q not found", ref.Name)
			}
			return nil, trace.Wrap(err)
		}

		return &windowsDesktopServiceCollection{services: []types.WindowsDesktopService{service}}, nil
	}

	services, err := apiclient.GetAllResources[types.WindowsDesktopService](ctx, client, &proto.ListResourcesRequest{ResourceType: types.KindWindowsDesktopService})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &windowsDesktopServiceCollection{services: services}, nil
}

func deleteWindowsDesktopService(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteWindowsDesktopService(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("windows desktop service %q has been deleted\n", ref.Name)
	return nil
}

type dynamicWindowsDesktopCollection struct {
	desktops []types.DynamicWindowsDesktop
}

func NewDynamicDesktopCollection(desktops []types.DynamicWindowsDesktop) Collection {
	return &dynamicWindowsDesktopCollection{
		desktops: desktops,
	}
}

func (c *dynamicWindowsDesktopCollection) Resources() (r []types.Resource) {
	for _, resource := range c.desktops {
		r = append(r, resource)
	}
	return r
}

func (c *dynamicWindowsDesktopCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, d := range c.desktops {
		labels := common.FormatLabels(d.GetAllLabels(), verbose)
		rows = append(rows, []string{d.GetName(), d.GetAddr(), d.GetDomain(), labels})
	}
	headers := []string{"Name", "Address", "AD Domain", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func createDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	wd, err := services.UnmarshalDynamicWindowsDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	dynamicDesktopClient := client.DynamicDesktopClient()
	if _, err := dynamicDesktopClient.CreateDynamicWindowsDesktop(ctx, wd); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.Force {
				return trace.AlreadyExists("dynamic windows desktop %q already exists", wd.GetName())
			}
			if _, err := dynamicDesktopClient.UpsertDynamicWindowsDesktop(ctx, wd); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("dynamic windows desktop %q has been updated\n", wd.GetName())
			return nil
		}
		return trace.Wrap(err)
	}

	fmt.Printf("dynamic windows desktop %q has been updated\n", wd.GetName())
	return nil
}

func updateDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	wd, err := services.UnmarshalDynamicWindowsDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	dynamicDesktopClient := client.DynamicDesktopClient()
	if _, err := dynamicDesktopClient.UpdateDynamicWindowsDesktop(ctx, wd); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("dynamic windows desktop %q has been updated\n", wd.GetName())
	return nil
}

func getDynamicWindowsDesktops(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	dynamicDesktopClient := client.DynamicDesktopClient()
	if ref.Name != "" {
		desktop, err := dynamicDesktopClient.GetDynamicWindowsDesktop(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &dynamicWindowsDesktopCollection{
			desktops: []types.DynamicWindowsDesktop{desktop},
		}, nil
	}

	desktops, err := stream.Collect(clientutils.Resources(ctx, dynamicDesktopClient.ListDynamicWindowsDesktops))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &dynamicWindowsDesktopCollection{desktops}, nil
}

func deleteDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DynamicDesktopClient().DeleteDynamicWindowsDesktop(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("dynamic windows desktop %q has been deleted\n", ref.Name)
	return nil
}

func dynamicWindowsDesktopHandler() Handler {
	return Handler{
		getHandler:    getDynamicWindowsDesktops,
		createHandler: createDynamicWindowsDesktop,
		updateHandler: updateDynamicWindowsDesktop,
		deleteHandler: deleteDynamicWindowsDesktop,
		description:   "A dynamically registered Windows desktop host.",
	}
}
