/*
Copyright 2022 Gravitational, Inc.

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

	"github.com/gravitational/reporting/types"
)

// NoopEnforcerService is a no-op enforcer service.
type NoopEnforcerService struct{}

// NewNoopEnforcerService returns a new no-op enforcer service.
func NewNoopEnforcerService() *NoopEnforcerService {
	return &NoopEnforcerService{}
}

// GetLicenseCheckResult returns the default heartbeat.
func (r *NoopEnforcerService) GetLicenseCheckResult(ctx context.Context) (*types.Heartbeat, error) {
	return types.NewHeartbeat(), nil
}
