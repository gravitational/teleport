/*
Copyright 2019 Gravitational, Inc.

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

package memory

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/trace"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestMemory(t *testing.T) {
	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
		cfg, err := test.ApplyOptions(options)

		if err != nil {
			return nil, nil, err
		}

		if cfg.ConcurrentBackend != nil {
			if _, ok := cfg.ConcurrentBackend.(*Memory); !ok {
				return nil, nil, trace.BadParameter("target is not a Memory backend")
			}
			return cfg.ConcurrentBackend, nil, nil
		}

		clock := clockwork.NewFakeClock()
		mem, err := New(Config{
			Clock:  clock,
			Mirror: cfg.MirrorMode,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return mem, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}
