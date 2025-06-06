package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getUser(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		users, err := client.GetUsers(ctx, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewUserCollection(users), nil
	}
	user, err := client.GetUser(ctx, rc.ref.Name, rc.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewUserCollection(services.Users{user}), nil
}

// createUser implements `tctl create user.yaml` command.
func (rc *ResourceCommand) createUser(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	user, err := services.UnmarshalUser(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	userName := user.GetName()
	existingUser, err := client.GetUser(ctx, userName, false)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)

	if exists {
		if !rc.force {
			return trace.AlreadyExists("user %q already exists", userName)
		}

		// Unmarshalling user sets createdBy to zero values which will overwrite existing data.
		// This field should not be allowed to be overwritten.
		user.SetCreatedBy(existingUser.GetCreatedBy())

		if _, err := client.UpsertUser(ctx, user); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %q has been updated\n", userName)

	} else {
		if _, err := client.CreateUser(ctx, user); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %q has been created\n", userName)
	}

	return nil
}

// updateUser implements `tctl create user.yaml` command.
func (rc *ResourceCommand) updateUser(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	user, err := services.UnmarshalUser(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user %q has been updated\n", user.GetName())

	return nil
}

func (rc *ResourceCommand) deleteUser(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteUser(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user %q has been deleted\n", rc.ref.Name)
	return nil
}
