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

	ec := EngineConfig{
		Context:      context.Background(),
		Clock:        clockwork.NewFakeClock(),
		Log:          logrus.StandardLogger(),
		Auth:         &testAuth{},
		Audit:        &testAudit{},
		AuthClient:   &auth.Client{},
		CloudClients: cloud.NewClients(),
	}

	// No engine is registered initially.
	engine, err := GetEngine("test", ec)
	require.Nil(t, engine)
	require.IsType(t, trace.NotFound(""), err)
	require.IsType(t, trace.NotFound(""), CheckEngines("test"))

	// Register a "test" engine.
	RegisterEngine(func(ec EngineConfig) Engine {
		return &testEngine{ec: ec}
	}, "test")

	// Create the registered engine instance.
	engine, err = GetEngine("test", ec)
	require.NoError(t, err)
	require.NotNil(t, engine)

	// Verify it's the one we registered.
	engineInst, ok := engine.(*testEngine)
	require.True(t, ok)
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
