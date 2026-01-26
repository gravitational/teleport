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
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
)

type certAuthorityCollection struct {
	cas []types.CertAuthority
}

func (a *certAuthorityCollection) Resources() (r []types.Resource) {
	for _, resource := range a.cas {
		r = append(r, resource)
	}
	return r
}

func (a *certAuthorityCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Cluster Name", "CA Type", "Fingerprint", "Role Map"})
	for _, a := range a.cas {
		for _, key := range a.GetTrustedSSHKeyPairs() {
			fingerprint, err := sshutils.AuthorizedKeyFingerprint(key.PublicKey)
			if err != nil {
				fingerprint = fmt.Sprintf("<bad key: %v>", err)
			}
			var roles string
			if a.GetType() == types.HostCA {
				roles = "N/A"
			} else {
				roles = fmt.Sprintf("%v", a.CombinedMapping())
			}
			t.AddRow([]string{
				a.GetClusterName(),
				string(a.GetType()),
				fingerprint,
				roles,
			})
		}
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func certAuthorityHandler() Handler {
	return Handler{
		getHandler:    getCertAuthority,
		deleteHandler: deleteCertAuthority,
		createHandler: createCertAuthority,
		singleton:     false,
		mfaRequired:   true,
		description:   "CA used by Teleport to sign certificates or access tokens. Each CA has a specific purpose.",
	}
}
func getCertAuthority(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	switch {
	// `tctl get cert_authority`.
	case ref.SubKind == "" && ref.Name == "":
		var allAuthorities []types.CertAuthority
		for _, caType := range types.CertAuthTypes {
			authorities, err := client.GetCertAuthorities(ctx, caType, opts.WithSecrets)
			if err != nil {
				if trace.IsBadParameter(err) {
					slog.WarnContext(ctx, "failed to get certificate authority; skipping", "error", err)
					continue
				}
				return nil, trace.Wrap(err)
			}
			allAuthorities = append(allAuthorities, authorities...)
		}
		return &certAuthorityCollection{cas: allAuthorities}, nil

	// Eg: `tctl get cert_authority/user`.
	case ref.SubKind == "":
		caType := ref.Name // ref.Name is set first.
		authorities, err := client.GetCertAuthorities(ctx, types.CertAuthType(caType), opts.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &certAuthorityCollection{cas: authorities}, nil

	// Eg: `tctl get cert_authority/user/example.com`.
	default:
		caType := ref.SubKind // ref.SubKind is set first.
		name := ref.Name
		id := types.CertAuthID{
			Type:       types.CertAuthType(caType),
			DomainName: name,
		}
		authority, err := client.GetCertAuthority(ctx, id, opts.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &certAuthorityCollection{cas: []types.CertAuthority{authority}}, nil
	}
}

func createCertAuthority(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
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
