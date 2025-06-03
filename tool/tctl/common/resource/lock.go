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

func (rc *ResourceCommand) getLock(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		locks, err := client.GetLocks(ctx, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewLockCollection(locks), nil
	}
	name := rc.ref.Name
	if rc.ref.SubKind != "" {
		name = rc.ref.SubKind + "/" + name
	}
	lock, err := client.GetLock(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewLockCollection([]types.Lock{lock}), nil
}

// createLock implements `tctl create lock.yaml` command.
func (rc *ResourceCommand) createLock(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	lock, err := services.UnmarshalLock(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if a lock of the name already exists.
	name := lock.GetName()
	_, err = client.GetLock(ctx, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := (err == nil)
	if !rc.force && exists {
		return trace.AlreadyExists("lock %q already exists", name)
	}

	if err := client.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("lock %q has been %s\n", name, UpsertVerb(exists, rc.force))
	return nil
}
