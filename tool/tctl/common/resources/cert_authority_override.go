// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/subca"
)

type certAuthorityOverrideCollection struct {
	overrides []*subcav1.CertAuthorityOverride
}

func (c *certAuthorityOverrideCollection) Resources() []types.Resource {
	ret := make([]types.Resource, len(c.overrides))
	for i, o := range c.overrides {
		ret[i] = types.Resource153ToLegacy(o)
	}
	return ret
}

func (c *certAuthorityOverrideCollection) WriteText(w io.Writer, verbose bool) error {
	// Mimics the CA table.
	// One entry per certificate override, or one entry per "empty" CA override.
	t := asciitable.MakeTable([]string{"Cluster Name", "CA Type", "Public Key Hash"})

	for _, caOverride := range c.overrides {
		overrides := caOverride.GetSpec().GetCertificateOverrides()
		if len(overrides) == 0 {
			t.AddRow([]string{
				caOverride.GetMetadata().GetName(),
				caOverride.GetSubKind(),
				"<none>",
			})
			continue
		}

		for _, o := range overrides {
			pkh := o.PublicKey
			if pkh == "" {
				cert, err := tlsutils.ParseCertificatePEM([]byte(o.Certificate))
				if err != nil {
					return trace.Wrap(err, "parse CA override certificate")
				}
				pkh = subca.HashCertificatePublicKey(cert)
			}
			t.AddRow([]string{
				caOverride.GetMetadata().GetName(),
				caOverride.GetSubKind(),
				pkh,
			})
		}
	}

	return trace.Wrap(t.WriteTo(w))
}

func certAuthorityOverrideHandler() Handler {
	return Handler{
		getHandler:    getCertAuthorityOverride,
		createHandler: createCertAuthorityOverride,
		updateHandler: updateCertAuthorityOverride,
		deleteHandler: deleteCertAuthorityOverride,
		description:   "CA overrides for Teleport CA certificates. Allows admins to chain Teleport CAs to an external trust root.",
	}
}

func getCertAuthorityOverride(
	ctx context.Context,
	authClient *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	// Defensive. Shouldn't happen. Only 3 forms of ref are possible:
	//   - kind ("tctl get ca_overrides")
	//   - kind + name ("tctl get ca_overrides/{ca_type}")
	//   - kind + subKind + name ("tctl get ca_overrides/{ca_type}/{cluster_name}")
	if ref.Kind != types.KindCertAuthorityOverride ||
		(ref.SubKind != "" && ref.Name == "") {
		return nil, trace.BadParameter("invalid ref: %#v", ref)
	}

	subCA := authClient.SubCAClient()

	// List.
	if ref.SubKind == "" && ref.Name == "" {
		caOverrides, err := stream.Collect(clientutils.Resources(
			ctx,
			func(ctx context.Context, pageSize int, pageToken string) ([]*subcav1.CertAuthorityOverride, string, error) {
				resp, err := subCA.ListCertAuthorityOverride(ctx, &subcav1.ListCertAuthorityOverrideRequest{
					PageSize:  int32(pageSize),
					PageToken: pageToken,
				})
				return resp.GetCaOverrides(), resp.GetNextPageToken(), trace.Wrap(err)
			},
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &certAuthorityOverrideCollection{overrides: caOverrides}, nil
	}

	// Get by {caType} or {caType}/{clusterName}.
	var caType, clusterName string
	if ref.SubKind != "" {
		caType = ref.SubKind
		clusterName = ref.Name
	} else {
		caType = ref.Name
	}

	// TODO(codingllama): Support cluster_name in ca_override Gets.
	resp, err := subCA.GetCertAuthorityOverride(ctx, &subcav1.GetCertAuthorityOverrideRequest{
		CaId: &subcav1.CertAuthorityOverrideID{
			CaType: caType,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caOverride := resp.CaOverride

	// Simulate a Get-by-name if the name was specified.
	if clusterName != "" && caOverride.Metadata.Name != clusterName {
		return nil, trace.NotFound("%s %s/%s not found", ref.Kind, caType, clusterName)
	}

	return &certAuthorityOverrideCollection{
		overrides: []*subcav1.CertAuthorityOverride{caOverride},
	}, nil
}

func createCertAuthorityOverride(ctx context.Context,
	authClient *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	caOverride, err := services.UnmarshalCertAuthorityOverride(
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	subCA := authClient.SubCAClient()

	var created *subcav1.CertAuthorityOverride
	var action string
	if opts.Force {
		resp, err1 := subCA.UpsertCertAuthorityOverride(ctx, &subcav1.UpsertCertAuthorityOverrideRequest{
			CaOverride:            caOverride,
			ForceImmediateDisable: true,
		})
		created = resp.GetCaOverride()
		err = err1
		action = "updated"
	} else {
		resp, err1 := subCA.CreateCertAuthorityOverride(ctx, &subcav1.CreateCertAuthorityOverrideRequest{
			CaOverride: caOverride,
		})
		created = resp.GetCaOverride()
		err = err1
		action = "created"
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%s %s/%s %s\n",
		types.KindCertAuthorityOverride,
		created.SubKind,
		created.Metadata.Name,
		action,
	)
	return nil
}

func updateCertAuthorityOverride(
	ctx context.Context,
	authClient *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	caOverride, err := services.UnmarshalCertAuthorityOverride(
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := authClient.
		SubCAClient().
		UpdateCertAuthorityOverride(ctx, &subcav1.UpdateCertAuthorityOverrideRequest{
			CaOverride: caOverride,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	updated := resp.CaOverride

	fmt.Printf("%s %s/%s updated\n",
		types.KindCertAuthorityOverride,
		updated.SubKind,
		updated.Metadata.Name,
	)
	return nil
}

func deleteCertAuthorityOverride(
	ctx context.Context,
	authClient *authclient.Client,
	ref services.Ref,
) error {
	if ref.Kind != types.KindCertAuthorityOverride ||
		ref.SubKind != "" ||
		ref.Name == "" {
		return trace.BadParameter(
			"%s deletes must the in the format %s/{ca_type}",
			types.KindCertAuthorityOverride,
			types.KindCertAuthorityOverride,
		)
	}
	caType := ref.Name

	// TODO(codingllama): Support cluster_name in ca_override Deletes.
	if _, err := authClient.
		SubCAClient().
		DeleteCertAuthorityOverride(ctx, &subcav1.DeleteCertAuthorityOverrideRequest{
			CaId: &subcav1.CertAuthorityOverrideID{
				CaType: caType,
			},
		}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%s %s deleted\n",
		types.KindCertAuthorityOverride,
		caType,
	)
	return nil
}
