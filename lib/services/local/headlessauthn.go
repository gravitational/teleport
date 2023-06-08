/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	item, err := MarshalHeadlessAuthenticationToItem(ha)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = s.Put(ctx, *item); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CompareAndSwapHeadlessAuthentication validates the new headless authentication and
// performs a compare and swap replacement on a headless authentication resource.
func (s *IdentityService) CompareAndSwapHeadlessAuthentication(ctx context.Context, old, new *types.HeadlessAuthentication) (*types.HeadlessAuthentication, error) {
	if err := services.ValidateHeadlessAuthentication(new); err != nil {
		return nil, trace.Wrap(err)
	}

	oldItem, err := MarshalHeadlessAuthenticationToItem(old)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newItem, err := MarshalHeadlessAuthenticationToItem(new)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.CompareAndSwap(ctx, *oldItem, *newItem)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return new, nil
}

// GetHeadlessAuthentication returns a headless authentication from the backend by name.
func (s *IdentityService) GetHeadlessAuthentication(ctx context.Context, name string) (*types.HeadlessAuthentication, error) {
	item, err := s.Get(ctx, headlessAuthenticationKey(name))
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
	rangeStart := headlessAuthenticationKey("")
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

// DeleteHeadlessAuthentication deletes a headless authentication from the backend by name.
func (s *IdentityService) DeleteHeadlessAuthentication(ctx context.Context, name string) error {
	err := s.Delete(ctx, headlessAuthenticationKey(name))
	return trace.Wrap(err)
}

// MarshalHeadlessAuthenticationToItem marshals a headless authentication to a backend.Item.
func MarshalHeadlessAuthenticationToItem(headlessAuthn *types.HeadlessAuthentication) (*backend.Item, error) {
	if err := headlessAuthn.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	value, err := utils.FastMarshal(headlessAuthn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &backend.Item{
		Key:     headlessAuthenticationKey(headlessAuthn.Metadata.Name),
		Value:   value,
		Expires: *headlessAuthn.Metadata.Expires,
	}, nil
}

// unmarshalHeadlessAuthenticationFromItem unmarshals a headless authentication from a backend.Item.
func unmarshalHeadlessAuthenticationFromItem(item *backend.Item) (*types.HeadlessAuthentication, error) {
	var headlessAuthn types.HeadlessAuthentication
	if err := utils.FastUnmarshal(item.Value, &headlessAuthn); err != nil {
		return nil, trace.Wrap(err, "error unmarshalling headless authentication from storage")
	}

	if err := headlessAuthn.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &headlessAuthn, nil
}

func headlessAuthenticationKey(name string) []byte {
	return backend.Key("headless_authentication", name)
}
