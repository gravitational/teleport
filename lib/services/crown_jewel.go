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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/lib/utils"
)

type CrownJewels interface {
	// ListCrownJewels returns the crown jewel of the company
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error)
	CreateCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error)
	UpdateCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error)
	UpsertCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error)
	DeleteCrownJewel(context.Context, string) error
	DeleteAllCrownJewels(context.Context) error
}

func MarshalCrownJewel(object *crownjewelv1.CrownJewel, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		object = proto.Clone(object).(*crownjewelv1.CrownJewel)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		object.Metadata.Id = 0
		object.Metadata.Revision = ""
	}
	data, err := utils.FastMarshal(object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

func UnmarshalCrownJewel(data []byte, opts ...MarshalOption) (*crownjewelv1.CrownJewel, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing crown jewel data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj crownjewelv1.CrownJewel
	if err := utils.FastUnmarshal(data, &obj); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if cfg.Revision != "" {
		obj.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		obj.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &obj, nil
}
