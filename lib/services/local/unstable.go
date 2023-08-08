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
	"github.com/gravitational/teleport/lib/backend"
)

// UnstableService is a catch-all for unstable backend operations related to migrations/compatibility
// that don't fit into, or merit the change of, one of the primary service interfaces.
type UnstableService struct {
	backend.Backend
	*AssertionReplayService
}

// NewUnstableService returns new unstable service instance.
func NewUnstableService(backend backend.Backend, assertion *AssertionReplayService) UnstableService {
	return UnstableService{backend, assertion}
}
