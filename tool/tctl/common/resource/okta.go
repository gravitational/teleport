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

var oktaImportRule = resource{
	getHandler:    getOktaImportRule,
	createHandler: createOktaImportRule,
	deleteHandler: deleteOktaImportRule,
}

func getOktaImportRule(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		importRule, err := client.OktaClient().GetOktaImportRule(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewOktaImportRuleCollection([]types.OktaImportRule{importRule}), nil
	}
	var resources []types.OktaImportRule
	nextKey := ""
	for {
		var importRules []types.OktaImportRule
		var err error
		importRules, nextKey, err = client.OktaClient().ListOktaImportRules(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, importRules...)
		if nextKey == "" {
			break
		}
	}
	return collections.NewOktaImportRuleCollection(resources), nil
}

func createOktaImportRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	importRule, err := services.UnmarshalOktaImportRule(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	exists := false
	if _, err = client.OktaClient().CreateOktaImportRule(ctx, importRule); err != nil {
		if trace.IsAlreadyExists(err) {
			exists = true
			_, err = client.OktaClient().UpdateOktaImportRule(ctx, importRule)
		}

		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("Okta import rule %q has been %s\n", importRule.GetName(), UpsertVerb(exists, opts.force))
	return nil
}

func deleteOktaImportRule(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.OktaClient().DeleteOktaImportRule(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Okta import rule %q has been deleted\n", ref.Name)
	return nil
}

var oktaAssignment = resource{
	getHandler: getOktaAssignment,
}

func getOktaAssignment(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		assignment, err := client.OktaClient().GetOktaAssignment(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewOktaAssignmentCollection([]types.OktaAssignment{assignment}), nil
	}
	var resources []types.OktaAssignment
	nextKey := ""
	for {
		var assignments []types.OktaAssignment
		var err error
		assignments, nextKey, err = client.OktaClient().ListOktaAssignments(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, assignments...)
		if nextKey == "" {
			break
		}
	}
	return collections.NewOktaAssignmentCollection(resources), nil
}

var userGroup = resource{
	getHandler:    getUserGroup,
	deleteHandler: deleteUserGroup,
}

func getUserGroup(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		userGroup, err := client.GetUserGroup(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewUserGroupCollection([]types.UserGroup{userGroup}), nil
	}
	var resources []types.UserGroup
	nextKey := ""
	for {
		var userGroups []types.UserGroup
		var err error
		userGroups, nextKey, err = client.ListUserGroups(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, userGroups...)
		if nextKey == "" {
			break
		}
	}
	return collections.NewUserGroupCollection(resources), nil
}

func deleteUserGroup(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteUserGroup(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("User group %q has been deleted\n", ref.Name)
	return nil
}
