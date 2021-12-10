/*
Copyright 2018-2019 Gravitational, Inc.

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

package lite

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
)

func TestLiteMem(t *testing.T) {
	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
		clock := clockwork.NewFakeClock()

		testCfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, err
		}

		backendCfg := Config{
			Memory:           true,
			PollStreamPeriod: 300 * time.Millisecond,
			Clock:            clock,
			Mirror:           testCfg.MirrorMode,
		}

		backend, err := NewWithConfig(context.Background(), backendCfg)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return backend, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}
