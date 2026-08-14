package common

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

type NodeOption func(*types.ServerV2)

func WithNodeSubkind(subkind string) NodeOption {
	return func(s *types.ServerV2) {
		s.SubKind = subkind
	}
}

func WithNodeLabel(key, value string) NodeOption {
	return func(s *types.ServerV2) {
		if s.Metadata.Labels == nil {
			s.Metadata.Labels = make(map[string]string)
		}
		s.Metadata.Labels[key] = value
	}
}

type NodeUpserter interface {
	UpsertNode(context.Context, types.Server) (*types.KeepAlive, error)
}

func CreateNode(ctx context.Context, dst NodeUpserter, name, hostname string, options ...NodeOption) (*types.ServerV2, error) {
	spec := types.ServerSpecV2{
		Hostname: hostname,
	}

	wrappedNode, err := types.NewNode(name, types.SubKindTeleportNode, spec, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawNode := wrappedNode.(*types.ServerV2)
	for _, option := range options {
		option(rawNode)
	}

	_, err = dst.UpsertNode(ctx, rawNode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rawNode, nil
}
