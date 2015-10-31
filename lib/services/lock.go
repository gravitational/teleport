/*
Copyright 2015 Gravitational, Inc.

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
package services

import (
	"time"

	"github.com/gravitational/teleport/lib/backend"
)

type LockService struct {
	backend backend.Backend
}

func NewLockService(backend backend.Backend) *LockService {
	return &LockService{backend}
}

// Grab a lock that will be released automatically in ttl time
func (s *LockService) AcquireLock(token string, ttl time.Duration) error {
	return s.backend.AcquireLock(token, ttl)
}

func (s *LockService) ReleaseLock(token string) error {
	return s.backend.ReleaseLock(token)
}
