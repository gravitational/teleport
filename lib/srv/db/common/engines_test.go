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

package common

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cloud"
)

// TestRegisterEngine verifies database engine registration.
func TestRegisterEngine(t *testing.T) {
	// Cleanup "test" engine in case this test is run in a loop.
	RegisterEngine(nil, "test")
	t.Cleanup(func() {
		RegisterEngine(nil, "test")
	})

	cloudClients, err := cloud.NewClients()
	require.NoError(t, err)
	ec := EngineConfig{
		Context:      context.Background(),
		Clock:        clockwork.NewFakeClock(),
		Log:          logrus.StandardLogger(),
		Auth:         &testAuth{},
		Audit:        &testAudit{},
		AuthClient:   &auth.Client{},
		CloudClients: cloudClients,
	}
	require.NoError(t, ec.CheckAndSetDefaults())

	// No engine is registered initially.
	db, err := types.NewDatabaseV3(types.Metadata{
		Name:   "prod",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "test",
		URI:      "uri",
	})
	require.NoError(t, err)

	engine, err := GetEngine(db, ec)
	require.Nil(t, engine)
	require.IsType(t, trace.NotFound(""), err)
	require.IsType(t, trace.NotFound(""), CheckEngines("test"))

	// Register a "test" engine.
	RegisterEngine(func(ec EngineConfig) Engine {
		return &testEngine{ec: ec}
	}, "test")

	// Create the registered engine instance.
	engine, err = GetEngine(db, ec)
	require.NoError(t, err)
	require.NotNil(t, engine)

	// Expect reporting engine wrapped around test engine
	repEngine, ok := engine.(*reportingEngine)
	require.True(t, ok)

	// Verify it's the one we registered.
	// The auth will be replaced with reporting auth internally, but we can unwrap the original auth.
	engineInst, ok := repEngine.engine.(*testEngine)
	require.True(t, ok)
	repAuth, ok := engineInst.ec.Auth.(*reportingAuth)
	require.True(t, ok)
	require.Equal(t, ec.Auth, repAuth.Auth)
	engineInst.ec.Auth = ec.Auth
	require.Equal(t, ec, engineInst.ec)
}

type testEngine struct {
	Engine
	ec EngineConfig
}

type testAudit struct {
	Audit
}

type testAuth struct {
	Auth
}
