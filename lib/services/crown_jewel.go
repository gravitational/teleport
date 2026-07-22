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
}

// MarshalCrownJewel marshals the CrownJewel object into a JSON byte array.
func MarshalCrownJewel(object *crownjewelv1.CrownJewel, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalCrownJewel unmarshals the CrownJewel object from a JSON byte array.
func UnmarshalCrownJewel(data []byte, opts ...MarshalOption) (*crownjewelv1.CrownJewel, error) {
	return UnmarshalProtoResource[*crownjewelv1.CrownJewel](data, opts...)
}
