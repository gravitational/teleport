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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type installerCollection struct {
	installers []types.Installer
}

func (c *installerCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.installers))
	for i, inst := range c.installers {
		r[i] = inst
	}
	return r
}

func (c *installerCollection) WriteText(w io.Writer, verbose bool) error {
	for _, inst := range c.installers {
		if _, err := fmt.Fprintf(w, "Script: %s\n----------\n", inst.GetName()); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(w, inst.GetScript()); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(w, "----------"); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func installerHandler() Handler {
	return Handler{
		getHandler:    getInstaller,
		createHandler: createInstaller,
		deleteHandler: deleteInstaller,
		singleton:     false,
		mfaRequired:   false,
		description:   "Installer scripts used by the Discovery Service for setting up Teleport on agents.",
	}
}

func getInstaller(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		// TODO(okraport): DELETE IN v21.0.0, replace with regular collect.
		installers, err := clientutils.CollectWithFallback(ctx, client.ListInstallers, client.GetInstallers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &installerCollection{installers: installers}, nil
	}
	inst, err := client.GetInstaller(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &installerCollection{installers: []types.Installer{inst}}, nil
}

func createInstaller(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	inst, err := services.UnmarshalInstaller(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.SetInstaller(ctx, inst)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("installer %q has been set\n", inst.GetName())
	return nil
}

func deleteInstaller(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	err := client.DeleteInstaller(ctx, ref.Name)
	if err != nil {
		return trace.Wrap(err)
	}
	if ref.Name == installers.InstallerScriptName {
		fmt.Printf("%s has been reset to a default value\n", ref.Name)
	} else {
		fmt.Printf("%s has been deleted\n", ref.Name)
	}
	return nil
}
