/*
Copyright 2023 Gravitational, Inc.

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

package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentityOutput_Render(t *testing.T) {
	cfg, err := newTestConfig("example.com")
	require.NoError(t, err)

	mockBot := newMockProvider(cfg)

	gd := newGoldenDestination(t)
	output := &IdentityOutput{
		Common: OutputCommon{
			Destination: gd.destination,
		},
	}
	require.NoError(t, output.CheckAndSetDefaults())
	require.NoError(t, output.Init())

	ctx := context.Background()
	ident := getTestIdent(t, "bot-test")
	require.NoError(t, output.Render(ctx, mockBot, ident))

	//gd.Assert(t)
	//gd.Update(t)
}
