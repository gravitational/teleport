/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package local

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// UpsertHeadlessAuthentication upserts a headless authentication in the backend.
func (s *IdentityService) UpsertHeadlessAuthentication(ctx context.Context, ha *types.HeadlessAuthentication) error {
	item, err := marshalHeadlessAuthenticationToItem(ha)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.Put(ctx, *item)
	return trace.Wrap(err)
}

// CompareAndSwapHeadlessAuthentication validates the new headless authentication and
// performs a compare and swap replacement on a headless authentication resource.
func (s *IdentityService) CompareAndSwapHeadlessAuthentication(ctx context.Context, old, new *types.HeadlessAuthentication) (*types.HeadlessAuthentication, error) {
	if err := services.ValidateHeadlessAuthentication(new); err != nil {
		return nil, trace.Wrap(err)
	}

	oldItem, err := marshalHeadlessAuthenticationToItem(old)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newItem, err := marshalHeadlessAuthenticationToItem(new)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.CompareAndSwap(ctx, *oldItem, *newItem)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return new, nil
}

// GetHeadlessAuthentication returns a headless authentication from the backend.
func (s *IdentityService) GetHeadlessAuthentication(ctx context.Context, username, name string) (*types.HeadlessAuthentication, error) {
	item, err := s.Get(ctx, headlessAuthenticationKey(username, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headlessAuthn, err := unmarshalHeadlessAuthenticationFromItem(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return headlessAuthn, nil
}

// GetHeadlessAuthentications returns all headless authentications from the backend.
func (s *IdentityService) GetHeadlessAuthentications(ctx context.Context) ([]*types.HeadlessAuthentication, error) {
	rangeStart := backend.ExactKey(headlessAuthenticationPrefix)
	rangeEnd := backend.RangeEnd(rangeStart)

	items, err := s.GetRange(ctx, rangeStart, rangeEnd, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headlessAuthns := make([]*types.HeadlessAuthentication, len(items.Items))
	for i, item := range items.Items {
		headlessAuthn, err := unmarshalHeadlessAuthenticationFromItem(&item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		headlessAuthns[i] = headlessAuthn
	}

	return headlessAuthns, nil
}

// DeleteHeadlessAuthentication deletes a headless authentication from the backend.
func (s *IdentityService) DeleteHeadlessAuthentication(ctx context.Context, username, name string) error {
	return trace.Wrap(s.Delete(ctx, headlessAuthenticationKey(username, name)))
}

// DeleteAllHeadlessAuthentications deletes all headless authentications from the backend.
func (s *IdentityService) DeleteAllHeadlessAuthentications(ctx context.Context) error {
	startKey := backend.ExactKey(headlessAuthenticationPrefix)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// marshalHeadlessAuthenticationToItem marshals a headless authentication to a backend.Item.
func marshalHeadlessAuthenticationToItem(headlessAuthn *types.HeadlessAuthentication) (*backend.Item, error) {
	value, err := marshalHeadlessAuthentication(headlessAuthn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &backend.Item{
		Key:     headlessAuthenticationKey(headlessAuthn.User, headlessAuthn.Metadata.Name),
		Value:   value,
		Expires: *headlessAuthn.Metadata.Expires,
	}, nil
}

// marshalHeadlessAuthentication marshals a headless authentication to JSON.
func marshalHeadlessAuthentication(ha *types.HeadlessAuthentication) ([]byte, error) {
	if err := ha.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return utils.FastMarshal(ha)
}

// unmarshalHeadlessAuthenticationFromItem unmarshals a headless authentication from a backend.Item.
func unmarshalHeadlessAuthenticationFromItem(item *backend.Item) (*types.HeadlessAuthentication, error) {
	return unmarshalHeadlessAuthentication(item.Value)
}

// unmarshalHeadlessAuthentication unmarshals a headless authentication from JSON.
func unmarshalHeadlessAuthentication(data []byte) (*types.HeadlessAuthentication, error) {
	var headlessAuthn types.HeadlessAuthentication
	if err := utils.FastUnmarshal(data, &headlessAuthn); err != nil {
		return nil, trace.Wrap(err, "error unmarshalling headless authentication from storage")
	}

	if err := headlessAuthn.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &headlessAuthn, nil
}

const headlessAuthenticationPrefix = "headless_authentication"

func headlessAuthenticationKey(username, name string) backend.Key {
	return backend.NewKey(headlessAuthenticationPrefix, usersPrefix, username, name)
}
