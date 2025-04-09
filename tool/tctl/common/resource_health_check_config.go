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

package common

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func (rc *ResourceCommand) createHealthCheckConfig(ctx context.Context, clt *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalHealthCheckConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	createFn := clt.CreateHealthCheckConfig
	if rc.IsForced() {
		createFn = clt.UpsertHealthCheckConfig
	}
	if _, err := createFn(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been created\n", in.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) updateHealthCheckConfig(ctx context.Context, clt *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalHealthCheckConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := clt.UpdateHealthCheckConfig(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) deleteHealthCheckConfig(ctx context.Context, clt *authclient.Client) error {
	if err := clt.DeleteHealthCheckConfig(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been deleted\n", rc.ref.Name)
	return nil
}
