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

// Note: this lives in tbot to avoid import cycles since this depends on the
// config/identity/destinations packages.

package tbot

import (
	"testing"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestLoadEmptyIdentity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := config.DestinationDirectory{
		Path: dir,
	}
	require.NoError(t, dest.CheckAndSetDefaults())

	_, err := identity.LoadIdentity(&dest, identity.BotKinds()...)
	require.Error(t, err)

	require.True(t, trace.IsNotFound(err))
}
