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

func (rc *ResourceCommand) getIntegration(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		ig, err := client.GetIntegration(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewIntegrationCollection([]types.Integration{ig}), nil
	}

	var resources []types.Integration
	var igs []types.Integration
	var err error
	var nextKey string
	for {
		igs, nextKey, err = client.ListIntegrations(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, igs...)
		if nextKey == "" {
			break
		}
	}
	return collections.NewIntegrationCollection(resources), nil
}

func (rc *ResourceCommand) createIntegration(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	integration, err := services.UnmarshalIntegration(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	existingIntegration, err := client.GetIntegration(ctx, integration.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)

	if exists {
		if !rc.force {
			return trace.AlreadyExists("Integration %q already exists", integration.GetName())
		}

		if err := existingIntegration.CanChangeStateTo(integration); err != nil {
			return trace.Wrap(err)
		}

		switch integration.GetSubKind() {
		case types.IntegrationSubKindAWSOIDC:
			existingIntegration.SetAWSOIDCIntegrationSpec(integration.GetAWSOIDCIntegrationSpec())
		case types.IntegrationSubKindGitHub:
			existingIntegration.SetGitHubIntegrationSpec(integration.GetGitHubIntegrationSpec())
		case types.IntegrationSubKindAWSRolesAnywhere:
			existingIntegration.SetAWSRolesAnywhereIntegrationSpec(integration.GetAWSRolesAnywhereIntegrationSpec())
		default:
			return trace.BadParameter("subkind %q is not supported", integration.GetSubKind())
		}

		if _, err := client.UpdateIntegration(ctx, existingIntegration); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Integration %q has been updated\n", integration.GetName())
		return nil
	}

	igV1, ok := integration.(*types.IntegrationV1)
	if !ok {
		return trace.BadParameter("unexpected Integration type %T", integration)
	}

	if _, err := client.CreateIntegration(ctx, igV1); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Integration %q has been created\n", integration.GetName())

	return nil
}
