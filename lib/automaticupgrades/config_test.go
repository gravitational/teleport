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

package automaticupgrades

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

func TestIsEnabled(t *testing.T) {
	t.Run("no env var returns false", func(t *testing.T) {
		t.Setenv(automaticUpgradesEnvar, "")
		require.False(t, IsEnabled())
	})
	t.Run("truthy value returns true", func(t *testing.T) {
		t.Setenv(automaticUpgradesEnvar, "1")
		require.True(t, IsEnabled())

		t.Setenv(automaticUpgradesEnvar, "TRUE")
		require.True(t, IsEnabled())
	})

	t.Run("falsy value returns false", func(t *testing.T) {
		t.Setenv(automaticUpgradesEnvar, "0")
		require.False(t, IsEnabled())

		t.Setenv(automaticUpgradesEnvar, "FALSE")
		require.False(t, IsEnabled())
	})

	t.Run("invalid value returns false and logs a warning message", func(t *testing.T) {
		hook := test.NewGlobal()
		defer hook.Reset()

		t.Setenv(automaticUpgradesEnvar, "foo")
		require.False(t, IsEnabled())

		require.Equal(t, 1, len(hook.Entries))
		require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
		require.Equal(t, `unexpected value for ENV:TELEPORT_AUTOMATIC_UPGRADES: strconv.ParseBool: parsing "foo": invalid syntax`, hook.LastEntry().Message)
	})
}
