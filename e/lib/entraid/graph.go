package entraid

import (
	"context"

	"github.com/gravitational/trace"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphsdkcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

type graphClient interface {
	IterateUsers(ctx context.Context, f func(models.Userable) bool) error
	IterateGroups(ctx context.Context, f func(models.Groupable) bool) error
	IterateGroupMembers(ctx context.Context, groupID string, f func(models.DirectoryObjectable) bool) error
}

// graphClientWrapper implements the graphClient interface
// by wrapping the concrete MS graph client.
type graphClientWrapper struct {
	client *msgraphsdk.GraphServiceClient
}

func (w *graphClientWrapper) IterateUsers(ctx context.Context, f func(models.Userable) bool) error {
	resp, err := w.client.Users().Get(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	pageIterator, err := msgraphsdkcore.NewPageIterator[models.Userable](resp, w.client.GetAdapter(), models.CreateUserCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pageIterator.Iterate(ctx, f))
}

func (w *graphClientWrapper) IterateGroups(ctx context.Context, f func(models.Groupable) bool) error {
	resp, err := w.client.Groups().Get(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	pageIterator, err := msgraphsdkcore.NewPageIterator[models.Groupable](resp, w.client.GetAdapter(), models.CreateGroupCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pageIterator.Iterate(ctx, f))
}

func (w *graphClientWrapper) IterateGroupMembers(ctx context.Context, groupID string, f func(models.DirectoryObjectable) bool) error {
	resp, err := w.client.Groups().ByGroupId(groupID).Members().Get(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	pageIterator, err := msgraphsdkcore.NewPageIterator[models.DirectoryObjectable](resp, w.client.GetAdapter(), models.CreateDirectoryObjectCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(pageIterator.Iterate(ctx, f))
}
