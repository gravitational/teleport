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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
)

// CrownJewels is the interface for managing crown jewel resources.
type CrownJewels interface {
	// ListCrownJewels returns the crown jewel resources.
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error)
	// GetCrownJewel returns the crown jewel resource by name.
	GetCrownJewel(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error)
	// CreateCrownJewel creates a new crown jewel resource.
	CreateCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error)
	// UpdateCrownJewel updates the crown jewel resource.
	UpdateCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error)
	// UpsertCrownJewel creates or updates the crown jewel resource.
	UpsertCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error)
	// DeleteCrownJewel deletes the crown jewel resource by name.
	DeleteCrownJewel(context.Context, string) error
	// DeleteAllCrownJewels deletes all crown jewel resources.
	DeleteAllCrownJewels(context.Context) error
}

// MarshalCrownJewel marshals the CrownJewel object into a JSON byte array.
func MarshalCrownJewel(object *crownjewelv1.CrownJewel, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		object = proto.Clone(object).(*crownjewelv1.CrownJewel)
		object.Metadata.Revision = ""
	}
	data, err := protojson.Marshal(object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalCrownJewel unmarshals the CrownJewel object from a JSON byte array.
func UnmarshalCrownJewel(data []byte, opts ...MarshalOption) (*crownjewelv1.CrownJewel, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing crown jewel data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj crownjewelv1.CrownJewel
	if err := protojson.Unmarshal(data, &obj); err != nil {
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
