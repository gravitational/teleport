/*
Copyright 2017 Gravitational, Inc.

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
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// UniversalSecondFactorService is responsible for managing universal second factor settings.
type UniversalSecondFactorService struct {
	backend.Backend
}

// NewUniversalSecondFactorService returns a new UniversalSecondFactorService.
func NewUniversalSecondFactorService(backend backend.Backend) *UniversalSecondFactorService {
	return &UniversalSecondFactorService{
		Backend: backend,
	}
}

// GetUniversalSecondFactor fetches the universal second factor settings
// from the backend and returns them.
func (s *UniversalSecondFactorService) GetUniversalSecondFactor() (services.UniversalSecondFactor, error) {
	data, err := s.GetVal([]string{"authentication", "preference"}, "u2f")
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no universal second factor settings")
		}
		return nil, trace.Wrap(err)
	}

	return services.GetUniversalSecondFactorMarshaler().Unmarshal(data)
}

// GetUniversalSecondFactor sets the universal second factor settings
// on the backend.
func (s *UniversalSecondFactorService) SetUniversalSecondFactor(settings services.UniversalSecondFactor) error {
	data, err := services.GetUniversalSecondFactorMarshaler().Marshal(settings)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertVal([]string{"authentication", "preference"}, "u2f", []byte(data), backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
