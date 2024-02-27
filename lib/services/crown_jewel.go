package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

type CrownJewels interface {
	// GetCrownJewel returns the crown jewel of the company
	GetCrownJewels(context.Context) ([]*types.CrownJewel, error)
	CreateCrownJewel(context.Context, *types.CrownJewel) (*types.CrownJewel, error)
	DeleteCrownJewel(context.Context, string) error
	DeleteAllCrownJewels(context.Context) error
}

func MarshalCrownJewel(crown *types.CrownJewel, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, crown))
}

func UnmarshalCrownJewel(data []byte, opts ...MarshalOption) (*types.CrownJewel, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing kube cluster data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var s types.CrownJewel
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
