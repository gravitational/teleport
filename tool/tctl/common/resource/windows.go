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

package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var windowsDesktopService = resource{
	getHandler:    getWindowsDesktopService,
	deleteHandler: deleteWindowsDesktopService,
}

func getWindowsDesktopService(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	services, err := client.GetWindowsDesktopServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewWindowsDesktopServiceCollection(services), nil
	}

	var out []types.WindowsDesktopService
	for _, service := range services {
		if service.GetName() == ref.Name {
			out = append(out, service)
		}
	}
	if len(out) == 0 {
		return nil, trace.NotFound("Windows desktop service %q not found", ref.Name)
	}
	return collections.NewWindowsDesktopServiceCollection(out), nil
}

func deleteWindowsDesktopService(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteWindowsDesktopService(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("windows desktop service %q has been deleted\n", ref.Name)
	return nil
}

var windowsDesktop = resource{
	getHandler:    getWindowsDesktop,
	createHandler: createWindowsDesktop,
	deleteHandler: deleteWindowsDesktop,
}

func getWindowsDesktop(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	desktops, err := client.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewWindowsDesktopCollection(desktops), nil
	}

	var out []types.WindowsDesktop
	for _, desktop := range desktops {
		if desktop.GetName() == ref.Name {
			out = append(out, desktop)
		}
	}
	if len(out) == 0 {
		return nil, trace.NotFound("Windows desktop %q not found", ref.Name)
	}
	return collections.NewWindowsDesktopCollection(out), nil
}

func createWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	wd, err := services.UnmarshalWindowsDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.UpsertWindowsDesktop(ctx, wd); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("windows desktop %q has been updated\n", wd.GetName())
	return nil
}

func deleteWindowsDesktop(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	desktops, err := client.GetWindowsDesktops(ctx,
		types.WindowsDesktopFilter{Name: ref.Name})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(desktops) == 0 {
		return trace.NotFound("no desktops with name %q were found", ref.Name)
	}
	deleted := 0
	var errs []error
	for _, desktop := range desktops {
		if desktop.GetName() == ref.Name {
			if err = client.DeleteWindowsDesktop(ctx, desktop.GetHostID(), ref.Name); err != nil {
				errs = append(errs, err)
				continue
			}
			deleted++
		}
	}
	if deleted == 0 {
		errs = append(errs,
			trace.Errorf("failed to delete any desktops with the name %q, %d were found",
				ref.Name, len(desktops)))
	}
	fmts := "%d windows desktops with name %q have been deleted"
	if err := trace.NewAggregate(errs...); err != nil {
		fmt.Printf(fmts+" with errors while deleting\n", deleted, ref.Name)
		return err
	}
	fmt.Printf(fmts+"\n", deleted, ref.Name)
	return nil
}

var dynamicWindowsDesktop = resource{
	getHandler:    getDynamicWindowsDesktop,
	createHandler: createDynamicWindowsDesktop,
	updateHandler: updateDynamicWindowsDesktop,
	deleteHandler: deleteDynamicWindowsDesktop,
}

func getDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	dynamicDesktopClient := client.DynamicDesktopClient()
	if ref.Name != "" {
		desktop, err := dynamicDesktopClient.GetDynamicWindowsDesktop(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewDynamicWindowsDesktopCollection([]types.DynamicWindowsDesktop{desktop}), nil
	}

	pageToken := ""
	desktops := make([]types.DynamicWindowsDesktop, 0, 100)
	for {
		d, next, err := dynamicDesktopClient.ListDynamicWindowsDesktops(ctx, 100, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if ref.Name == "" {
			desktops = append(desktops, d...)
		} else {
			for _, desktop := range desktops {
				if desktop.GetName() == ref.Name {
					desktops = append(desktops, desktop)
				}
			}
		}
		pageToken = next
		if next == "" {
			break
		}
	}

	return collections.NewDynamicWindowsDesktopCollection(desktops), nil
}

func createDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	wd, err := services.UnmarshalDynamicWindowsDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	dynamicDesktopClient := client.DynamicDesktopClient()
	if _, err := dynamicDesktopClient.CreateDynamicWindowsDesktop(ctx, wd); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.force {
				return trace.AlreadyExists("application %q already exists", wd.GetName())
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

func updateDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DynamicDesktopClient().DeleteDynamicWindowsDesktop(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("dynamic windows desktop %q has been deleted\n", ref.Name)
	return nil
}
