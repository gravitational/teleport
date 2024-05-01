/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
		return nil, trace.BadParameter("missing crown jewel data")
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
