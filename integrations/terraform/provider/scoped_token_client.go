package provider

import (
	"context"

	"github.com/gravitational/teleport/api/client"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"

	"github.com/gravitational/trace"
)

type scopedTokenClient struct {
	client *client.Client
}

func (r scopedTokenClient) Get(ctx context.Context, req GetResourceRequest[NameIdentifier]) (*joiningv1.ScopedToken, error) {
	scopedToken, err := r.client.GetScopedToken(ctx, req.Identifier.Name, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return scopedToken, nil
}

func (r scopedTokenClient) Create(ctx context.Context, req *joiningv1.ScopedToken) error {
	_, err := r.client.CreateScopedToken(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r scopedTokenClient) Upsert(ctx context.Context, req *joiningv1.ScopedToken) error {
	_, err := r.client.UpsertScopedToken(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r scopedTokenClient) Delete(ctx context.Context, req NameIdentifier) error {
	return trace.Wrap(r.client.DeleteScopedToken(ctx, req.Name))
}
