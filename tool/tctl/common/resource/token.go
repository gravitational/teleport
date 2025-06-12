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

var token = resource{
	getHandler:    getToken,
	createHandler: createToken,
	deleteHandler: deleteToken,
}

func getToken(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name == "" {
		tokens, err := client.GetTokens(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewTokenCollection(tokens), nil
	}
	token, err := client.GetToken(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewTokenCollection([]types.ProvisionToken{token}), nil
}

func createToken(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	token, err := services.UnmarshalProvisionToken(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.UpsertToken(ctx, token)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("provision_token %q has been created\n", token.GetName())
	return nil
}

func deleteToken(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteToken(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("token %q has been deleted\n", ref.Name)
	return nil
}
