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

func (rc *ResourceCommand) getCertAuthority(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	getAll := rc.ref.SubKind == "" && rc.ref.Name == ""
	if getAll {
		var allAuthorities []types.CertAuthority
		for _, caType := range types.CertAuthTypes {
			authorities, err := client.GetCertAuthorities(ctx, caType, rc.withSecrets)
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

	id := types.CertAuthID{Type: types.CertAuthType(rc.ref.SubKind), DomainName: rc.ref.Name}
	authority, err := client.GetCertAuthority(ctx, id, rc.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAuthorityCollection([]types.CertAuthority{authority}), nil
}

// createCertAuthority creates certificate authority
func (rc *ResourceCommand) createCertAuthority(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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

func (rc *ResourceCommand) deleteCertAuthority(ctx context.Context, client *authclient.Client) error {
	if rc.ref.SubKind == "" || rc.ref.Name == "" {
		return trace.BadParameter(
			"full %s path must be specified (e.g. '%s/%s/clustername')",
			types.KindCertAuthority, types.KindCertAuthority, types.HostCA,
		)
	}
	err := client.DeleteCertAuthority(ctx, types.CertAuthID{
		Type:       types.CertAuthType(rc.ref.SubKind),
		DomainName: rc.ref.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s '%s/%s' has been deleted\n", types.KindCertAuthority, rc.ref.SubKind, rc.ref.Name)
	return nil
}
