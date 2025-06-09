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
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var certAuthority = resource{
	getHandler:    getCertAuthority,
	createHandler: createCertAuthority,
	deleteHandler: deleteCertAuthority,
}

func getCertAuthority(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	getAll := ref.SubKind == "" && ref.Name == ""
	if getAll {
		var allAuthorities []types.CertAuthority
		for _, caType := range types.CertAuthTypes {
			authorities, err := client.GetCertAuthorities(ctx, caType, opts.withSecrets)
			if err != nil {
				if trace.IsBadParameter(err) {
					slog.WarnContext(ctx, "failed to get certificate authority; skipping", "error", err)
					continue
				}
				return nil, trace.Wrap(err)
			}
			allAuthorities = append(allAuthorities, authorities...)
		}
		return collections.NewAuthorityCollection(allAuthorities), nil
	}

	id := types.CertAuthID{Type: types.CertAuthType(ref.SubKind), DomainName: ref.Name}
	authority, err := client.GetCertAuthority(ctx, id, opts.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAuthorityCollection([]types.CertAuthority{authority}), nil
}

// createCertAuthority creates certificate authority
func createCertAuthority(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	certAuthority, err := services.UnmarshalCertAuthority(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.UpsertCertAuthority(ctx, certAuthority); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("certificate authority %q has been updated\n", certAuthority.GetName())
	return nil
}

func deleteCertAuthority(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if ref.SubKind == "" || ref.Name == "" {
		return trace.BadParameter(
			"full %s path must be specified (e.g. '%s/%s/clustername')",
			types.KindCertAuthority, types.KindCertAuthority, types.HostCA,
		)
	}
	err := client.DeleteCertAuthority(ctx, types.CertAuthID{
		Type:       types.CertAuthType(ref.SubKind),
		DomainName: ref.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s '%s/%s' has been deleted\n", types.KindCertAuthority, ref.SubKind, ref.Name)
	return nil
}
