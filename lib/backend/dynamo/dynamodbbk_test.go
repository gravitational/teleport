// +build dynamodb

/*
Copyright 2015-2018 Gravitational, Inc.

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

package dynamo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

const tableName = "teleport.dynamo.test"

func TestDynamoDB(t *testing.T) {
	dynamoCfg := map[string]interface{}{
		"table_name":         tableName,
		"poll_stream_period": 300 * time.Millisecond,
	}

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
		testCfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if testCfg.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		// This would seem to be a bad thing for dynamo to omit
		if testCfg.ConcurrentBackend != nil {
			return nil, nil, test.ErrConcurrentAccessNotSupported
		}

		uut, err := New(context.Background(), dynamoCfg)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		clock := clockwork.NewFakeClock()
		uut.clock = clock
		return uut, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}
