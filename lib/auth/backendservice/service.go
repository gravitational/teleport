package backendservice

import (
	"bytes"
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	backendv1alpha "github.com/gravitational/teleport/api/gen/proto/go/backend/v1alpha"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
)

func NewServer(authorizer authz.Authorizer, backend backend.Backend) backendv1alpha.BackendServiceServer {
	return &server{
		auth: authorizer,
		bk:   backend,
	}
}

type server struct {
	backendv1alpha.UnsafeBackendServiceServer

	auth authz.Authorizer
	bk   backend.Backend
}

func (b *server) requireAdmin(ctx context.Context) error {
	authContext, err := b.auth.Authorize(ctx)
	if err != nil {
		return trace.AccessDenied("builtin Admin role is required")
	}
	if _, ok := authContext.Identity.(authz.BuiltinRole); !ok {
		return trace.AccessDenied("builtin Admin role is required")
	}
	if !authContext.Checker.HasRole(string(types.RoleAdmin)) {
		return trace.AccessDenied("builtin Admin role is required")
	}
	return nil
}

var _ backendv1alpha.BackendServiceServer = (*server)(nil)

// Read implements backendv1alpha.BackendServiceServer.
func (b *server) Read(ctx context.Context, req *backendv1alpha.ReadRequest) (*backendv1alpha.ReadResponse, error) {
	if err := b.requireAdmin(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	protoItem := func(i backend.Item) *backendv1alpha.Item {
		var expires *timestamppb.Timestamp
		if !i.Expires.IsZero() {
			expires = timestamppb.New(i.Expires)
		}
		return &backendv1alpha.Item{
			Key:     i.Key,
			Value:   i.Value,
			Expires: expires,
		}
	}

	if bytes.Equal(req.GetStartKey(), req.GetEndKey()) {
		i, err := b.bk.Get(ctx, req.GetStartKey())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &backendv1alpha.ReadResponse{
			Items: []*backendv1alpha.Item{protoItem(*i)},
		}, nil
	}

	getResult, err := b.bk.GetRange(ctx, req.GetStartKey(), req.GetEndKey(), int(req.GetLimit()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &backendv1alpha.ReadResponse{
		Items: make([]*backendv1alpha.Item, 0, len(getResult.Items)),
	}
	for _, i := range getResult.Items {
		resp.Items = append(resp.Items, protoItem(i))
	}

	return resp, nil
}

// Write implements backendv1.BackendServiceServer.
func (b *server) Write(ctx context.Context, req *backendv1alpha.WriteRequest) (*backendv1alpha.WriteResponse, error) {
	if err := b.requireAdmin(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	switch act := req.GetOperation().GetAction().(type) {
	case *backendv1alpha.WriteOperation_Put:
		switch cond := act.Put.GetCondition().(type) {
		case *backendv1alpha.PutAction_Any:
			if _, err := b.bk.Put(ctx, backend.Item{
				Key:     req.GetOperation().GetKey(),
				Value:   act.Put.GetValue(),
				Expires: act.Put.GetExpires().AsTime(),
			}); err != nil {
				return nil, trace.Wrap(err)
			}
			return &backendv1alpha.WriteResponse{}, nil

		case *backendv1alpha.PutAction_Exists:
			if cond.Exists {
				if _, err := b.bk.Update(ctx, backend.Item{
					Key:     req.GetOperation().GetKey(),
					Value:   act.Put.GetValue(),
					Expires: act.Put.GetExpires().AsTime(),
				}); err != nil {
					return nil, trace.Wrap(err)
				}
				return &backendv1alpha.WriteResponse{}, nil
			} else {
				if _, err := b.bk.Create(ctx, backend.Item{
					Key:     req.GetOperation().GetKey(),
					Value:   act.Put.GetValue(),
					Expires: act.Put.GetExpires().AsTime(),
				}); err != nil {
					return nil, trace.Wrap(err)
				}
				return &backendv1alpha.WriteResponse{}, nil
			}

		case *backendv1alpha.PutAction_ValueIs:
			if _, err := b.bk.CompareAndSwap(ctx,
				backend.Item{
					Key:   req.GetOperation().GetKey(),
					Value: cond.ValueIs,
				},
				backend.Item{
					Key:     req.GetOperation().GetKey(),
					Value:   act.Put.GetValue(),
					Expires: act.Put.GetExpires().AsTime(),
				}); err != nil {
				return nil, trace.Wrap(err)
			}
			return &backendv1alpha.WriteResponse{}, nil

		case *backendv1alpha.PutAction_RevisionIs:
			return nil, trace.NotImplemented("Put with RevisionIs is not implemented")
		default:
			return nil, trace.NotImplemented("unknown or missing condition for Put")
		}

	case *backendv1alpha.WriteOperation_KeepAlive:
		switch act.KeepAlive.GetCondition().(type) {
		case *backendv1alpha.KeepAliveAction_Exists:
			if err := b.bk.KeepAlive(ctx, backend.Lease{
				Key: req.GetOperation().GetKey(),
			}, act.KeepAlive.GetExpires().AsTime()); err != nil {
				return nil, trace.Wrap(err)
			}
			return &backendv1alpha.WriteResponse{}, nil

		case *backendv1alpha.KeepAliveAction_RevisionIs:
			return nil, trace.NotImplemented("KeepAlive with RevisionIs is not implemented")
		default:
			return nil, trace.NotImplemented("unknown or missing condition for KeepAlive")
		}

	case *backendv1alpha.WriteOperation_Delete:
		switch act.Delete.GetCondition().(type) {
		case *backendv1alpha.DeleteAction_Exists:
			if err := b.bk.Delete(ctx, req.GetOperation().GetKey()); err != nil {
				return nil, trace.Wrap(err)
			}
			return &backendv1alpha.WriteResponse{}, nil

		case *backendv1alpha.DeleteAction_RevisionIs:
			return nil, trace.NotImplemented("Delete with RevisionIs is not implemented")
		default:
			return nil, trace.NotImplemented("unknown or missing condition for Delete")
		}

	default:
		return nil, trace.NotImplemented("unknown or missing operation for Write")
	}
}

// DeleteRange implements backendv1alpha.BackendServiceServer.
func (b *server) DeleteRange(ctx context.Context, req *backendv1alpha.DeleteRangeRequest) (*backendv1alpha.DeleteRangeResponse, error) {
	if err := b.requireAdmin(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := b.bk.DeleteRange(ctx, req.GetStartKey(), req.GetEndKey()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &backendv1alpha.DeleteRangeResponse{}, nil
}

// MultiWrite implements backendv1alpha.BackendServiceServer.
func (b *server) MultiWrite(ctx context.Context, req *backendv1alpha.MultiWriteRequest) (*backendv1alpha.MultiWriteResponse, error) {
	if err := b.requireAdmin(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, trace.NotImplemented("MultiWrite is not implemented")
}
