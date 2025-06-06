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

func (rc *ResourceCommand) getToken(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		tokens, err := client.GetTokens(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewTokenCollection(tokens), nil
	}
	token, err := client.GetToken(ctx, rc.ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewTokenCollection([]types.ProvisionToken{token}), nil
}

func (rc *ResourceCommand) createToken(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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

func (rc *ResourceCommand) deleteToken(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteToken(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("token %q has been deleted\n", rc.ref.Name)
	return nil
}
