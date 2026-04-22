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

	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type linuxDesktopCollection struct {
	desktops []*linuxdesktopv1.LinuxDesktop
}

func linuxDesktopHandler() Handler {
	return Handler{
		getHandler:    getLinuxDesktops,
		createHandler: createLinuxDesktop,
		updateHandler: updateLinuxDesktop,
		deleteHandler: deleteLinuxDesktop,
		singleton:     false,
		mfaRequired:   false,
		description:   "A Linux remote desktop protected by Teleport",
	}
}

func (c *linuxDesktopCollection) Resources() (r []types.Resource) {
	for _, resource := range c.desktops {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *linuxDesktopCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, d := range c.desktops {
		labels := common.FormatLabels(d.Metadata.Labels, verbose)
		rows = append(rows, []string{d.Metadata.Name, d.Spec.Addr, d.Spec.Hostname, labels})
	}
	headers := []string{"Name", "Address", "Hostname", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func getLinuxDesktops(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	linuxDesktopClient := client.LinuxDesktopClient()

	if ref.Name != "" {
		desktop, err := linuxDesktopClient.GetLinuxDesktop(ctx, ref.Name)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("Linux desktop %q not found", ref.Name)
			}
			return nil, trace.Wrap(err)
		}
		return &linuxDesktopCollection{desktops: []*linuxdesktopv1.LinuxDesktop{desktop}}, nil
	}

	desktops, err := stream.Collect(clientutils.Resources(ctx, linuxDesktopClient.ListLinuxDesktops))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &linuxDesktopCollection{desktops: desktops}, nil
}

func deleteLinuxDesktop(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	linuxDesktopClient := client.LinuxDesktopClient()
	if err := linuxDesktopClient.DeleteLinuxDesktop(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("linux desktop %q has been deleted\n", ref.Name)
	return nil
}

func createLinuxDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	desktop, err := services.UnmarshalLinuxDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	linuxDesktopClient := client.LinuxDesktopClient()
	if _, err := linuxDesktopClient.CreateLinuxDesktop(ctx, desktop); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.Force {
				return trace.AlreadyExists("linux desktop %q already exists", desktop.Metadata.Name)
			}
			if _, err := linuxDesktopClient.UpsertLinuxDesktop(ctx, desktop); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("linux desktop %q has been updated\n", desktop.Metadata.Name)
			return nil
		}
		return trace.Wrap(err)
	}

	fmt.Printf("linux desktop %q has been created\n", desktop.Metadata.Name)
	return nil
}

func updateLinuxDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	desktop, err := services.UnmarshalLinuxDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	linuxDesktopClient := client.LinuxDesktopClient()
	if _, err := linuxDesktopClient.UpdateLinuxDesktop(ctx, desktop); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("linux desktop %q has been updated\n", desktop.Metadata.Name)
	return nil
}
