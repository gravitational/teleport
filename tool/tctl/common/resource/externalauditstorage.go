package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var externalAuditStorage = resource{
	getHandler:    getExternalAuditStorage,
	createHandler: createExternalAuditStorage,
	deleteHandler: deleteExternalAuditStorage,
}

// createExternalAuditStorage implements `tctl create external_audit_storage` command.
func createExternalAuditStorage(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	draft, err := services.UnmarshalExternalAuditStorage(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	externalAuditClient := client.ExternalAuditStorageClient()
	if opts.force {
		if _, err := externalAuditClient.UpsertDraftExternalAuditStorage(ctx, draft); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("External Audit Storage configuration has been updated\n")
	} else {
		if _, err := externalAuditClient.CreateDraftExternalAuditStorage(ctx, draft); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("External Audit Storage configuration has been created\n")
	}
	return nil
}

func getExternalAuditStorage(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	out := []*externalauditstorage.ExternalAuditStorage{}
	name := ref.Name
	switch name {
	case "":
		cluster, err := client.ExternalAuditStorageClient().GetClusterExternalAuditStorage(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
		} else {
			out = append(out, cluster)
		}
		draft, err := client.ExternalAuditStorageClient().GetDraftExternalAuditStorage(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
		} else {
			out = append(out, draft)
		}
		return collections.NewExternalAuditStorageCollection(out), nil
	case types.MetaNameExternalAuditStorageCluster:
		cluster, err := client.ExternalAuditStorageClient().GetClusterExternalAuditStorage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewExternalAuditStorageCollection([]*externalauditstorage.ExternalAuditStorage{cluster}), nil
	case types.MetaNameExternalAuditStorageDraft:
		draft, err := client.ExternalAuditStorageClient().GetDraftExternalAuditStorage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewExternalAuditStorageCollection([]*externalauditstorage.ExternalAuditStorage{draft}), nil
	default:
		return nil, trace.BadParameter("unsupported resource name for external_audit_storage, valid for get are: '', %q, %q", types.MetaNameExternalAuditStorageDraft, types.MetaNameExternalAuditStorageCluster)
	}
}

func deleteExternalAuditStorage(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if ref.Name == types.MetaNameExternalAuditStorageCluster {
		if err := client.ExternalAuditStorageClient().DisableClusterExternalAuditStorage(ctx); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("cluster External Audit Storage configuration has been disabled\n")
	} else {
		if err := client.ExternalAuditStorageClient().DeleteDraftExternalAuditStorage(ctx); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("draft External Audit Storage configuration has been deleted\n")
	}
	return nil
}
