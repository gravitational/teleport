/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
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
		Context:           context.Background(),
		Clock:             clockwork.NewFakeClock(),
		Log:               slog.Default(),
		Auth:              &testAuth{},
		Audit:             &testAudit{},
		AuthClient:        &authclient.Client{},
		AWSConfigProvider: &mocks.AWSConfigProvider{},
		CloudClients:      cloudClients,
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
