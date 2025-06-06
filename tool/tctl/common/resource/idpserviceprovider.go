package resource

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createSAMLIdPServiceProvider(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	// Create services.SAMLIdPServiceProvider from raw YAML to extract the service provider name.
	sp, err := services.UnmarshalSAMLIdPServiceProvider(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if sp.GetEntityDescriptor() != "" {
		// verify that entity descriptor parses
		ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
		if err != nil {
			return trace.BadParameter("invalid entity descriptor for SAML IdP Service Provider %q: %v", sp.GetEntityID(), err)
		}

		// issue warning about unsupported ACS bindings.
		if err := services.FilterSAMLEntityDescriptor(ed, false /* quiet */); err != nil {
			slog.WarnContext(ctx, "Entity descriptor for SAML IdP service provider contains unsupported ACS bindings",
				"entity_id", sp.GetEntityID(),
				"error", err,
			)
		}
	}

	serviceProviderName := sp.GetName()

	exists := false
	if err = client.CreateSAMLIdPServiceProvider(ctx, sp); err != nil {
		if trace.IsAlreadyExists(err) {
			exists = true
			err = client.UpdateSAMLIdPServiceProvider(ctx, sp)
		}

		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("SAML IdP service provider %q has been %s\n", serviceProviderName, UpsertVerb(exists, rc.IsForced()))
	return nil
}

func (rc *ResourceCommand) getSAMLIdPServiceProvider(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		serviceProvider, err := client.GetSAMLIdPServiceProvider(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewSAMLIdPServiceProviderCollection([]types.SAMLIdPServiceProvider{serviceProvider}), nil
	}
	var resources []types.SAMLIdPServiceProvider
	nextKey := ""
	for {
		var sps []types.SAMLIdPServiceProvider
		var err error
		sps, nextKey, err = client.ListSAMLIdPServiceProviders(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, sps...)
		if nextKey == "" {
			break
		}
	}
	return collections.NewSAMLIdPServiceProviderCollection(resources), nil
}

func (rc *ResourceCommand) deleteSAMLIdPServiceProvider(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteSAMLIdPServiceProvider(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SAML IdP service provider %q has been deleted\n", rc.ref.Name)
	return nil
}
