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
