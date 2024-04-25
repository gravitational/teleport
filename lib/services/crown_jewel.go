package services

import (
	"context"

	"github.com/gravitational/trace"

	crownjewelclient "github.com/gravitational/teleport/api/client/crownjewel"
	"github.com/gravitational/teleport/api/types/crownjewel"
	"github.com/gravitational/teleport/lib/utils"
)

var _ CrownJewels = (*crownjewelclient.Client)(nil)

type CrownJewels interface {
	// ListCrownJewels returns the crown jewel of the company
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewel.CrownJewel, error)
	CreateCrownJewel(context.Context, *crownjewel.CrownJewel) (*crownjewel.CrownJewel, error)
	UpdateCrownJewel(context.Context, *crownjewel.CrownJewel) (*crownjewel.CrownJewel, error)
	DeleteCrownJewel(context.Context, string) error
	DeleteAllCrownJewels(context.Context) error
}

func MarshalCrownJewel(crown *crownjewel.CrownJewel, opts ...MarshalOption) ([]byte, error) {
	if err := crown.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *crown
		copy.SetResourceID(0)
		copy.SetRevision("")
		crown = &copy
	}
	return utils.FastMarshal(crown)
}

func UnmarshalCrownJewel(data []byte, opts ...MarshalOption) (*crownjewel.CrownJewel, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing kube cluster data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var s crownjewel.CrownJewel
	if err := utils.FastUnmarshal(data, &s); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		s.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		s.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		s.SetExpiry(cfg.Expires)
	}
	return &s, nil
}
