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
package db

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestDiagnose(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withRDSMySQL("mysql-rds", "root", "bad-token"))
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	diagChan := make(chan error)
	testCtx.server.cfg.OnDiagnose = func(database types.Database, err error) {
		diagChan <- err
	}
	waitForIAMAuthErrorDiagnosed := func(t *testing.T) {
		select {
		case diagedErr := <-diagChan:
			require.True(t, common.IsIAMAuthError(diagedErr), "expect error is common.IAMAuthError")
		case <-time.After(5 * time.Second):
			require.Fail(t, "Didn't receive diagnose event.")
		}
	}

	go testCtx.startHandlingConnections()

	// Diagnose IAM auth error.
	_, err := testCtx.mysqlClient("alice", "mysql-rds", "root")
	require.Error(t, err)
	waitForIAMAuthErrorDiagnosed(t)

	// Diagnose should be rate limited per database.
	err = testCtx.server.addDiagnoseTask(testCtx.mysql["mysql-rds"].resource, common.NewIAMAuthError("auth error"))
	require.True(t, trace.IsLimitExceeded(err), "expect error is trace.LimitExceeded")

	// Advance time to see rate limit is lifted.
	testCtx.advanceClock(time.Hour)
	_, err = testCtx.mysqlClient("alice", "mysql-rds", "root")
	require.Error(t, err)
	waitForIAMAuthErrorDiagnosed(t)
}
