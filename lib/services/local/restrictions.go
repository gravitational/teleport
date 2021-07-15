
/*
Copyright 2021 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// RestrictionsService manages restrictions to be enforced by restricted shell
type RestrictionsService struct {
	backend.Backend
}

// NewRestrictionsService creates a new RestrictionsService
func NewRestrictionsService(backend backend.Backend) *RestrictionsService {
	return &RestrictionsService{Backend: backend}
}

// SetNetworkRestrictions upserts NetworkRestrictions
func (s *RestrictionsService) SetNetworkRestrictions(ctx context.Context, nr types.NetworkRestrictions) error {
	value, err := services.MarshalNetworkRestrictions(nr)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key: backend.Key(restrictionsPrefix, network),
		Value: value,
		Expires: nr.Expiry(),
		ID: nr.GetResourceID(),
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *RestrictionsService) GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error) {
	item, err := s.Get(context.TODO(), backend.Key(restrictionsPrefix, network))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalNetworkRestrictions(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
}

// SetNetworkRestrictions upserts NetworkRestrictions
func (s *RestrictionsService) DeleteNetworkRestrictions(ctx context.Context) error {
	return trace.Wrap(s.Delete(ctx, backend.Key(restrictionsPrefix, network)))
}

const (
	restrictionsPrefix = "restrictions"
	network = "network"
)
