package services

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	crownjewelclient "github.com/gravitational/teleport/api/client/crownjewel"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/lib/utils"
)

var _ CrownJewels = (*crownjewelclient.Client)(nil)

type CrownJewels interface {
	// ListCrownJewels returns the crown jewel of the company
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error)
	CreateCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error)
	UpdateCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error)
	DeleteCrownJewel(context.Context, string) error
	DeleteAllCrownJewels(context.Context) error
}

func MarshalCrownJewel(crown *crownjewelv1.CrownJewel, opts ...MarshalOption) ([]byte, error) {
	return utils.FastMarshal(crown)
}

func UnmarshalCrownJewel(data []byte, opts ...MarshalOption) (*crownjewelv1.CrownJewel, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing kube cluster data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var s crownjewelv1.CrownJewel
	if err := utils.FastUnmarshal(data, &s); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if cfg.Revision != "" {
		s.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		s.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &s, nil
}
