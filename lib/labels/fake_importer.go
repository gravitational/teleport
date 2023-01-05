/*
Copyright 2020 Gravitational, Inc.

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

// Package labels provides a way to get dynamic labels. Used by SSH, App,
// and Kubernetes servers.
package labels

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/api/types"
)

// verify importer interface compliance
var _ Importer = (*FakeImporter)(nil)

// NewFakeImporter creates an importer that serves a static label set.
func NewFakeImporter(labels map[string]string) *FakeImporter {
	return &FakeImporter{
		labels: labels,
	}
}

// FakeImporter is a helper for serving a static label set as a labels.Importer (used
// in tests).
type FakeImporter struct {
	mu     sync.Mutex
	labels map[string]string
}

func (s *FakeImporter) SetLabels(labels map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.labels = labels
}

func (s *FakeImporter) Get() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := make(map[string]string, len(s.labels))
	for k, v := range s.labels {
		l[k] = v
	}
	return l
}

func (s *FakeImporter) Apply(r types.ResourceWithLabels) {
	labels := s.Get()
	for k, v := range r.GetStaticLabels() {
		labels[k] = v
	}
	r.SetStaticLabels(labels)
}

func (s *FakeImporter) Sync(_ context.Context) error {
	return nil
}

func (s *FakeImporter) Start(_ context.Context) {}
