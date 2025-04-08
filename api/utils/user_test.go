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

package utils

import (
	"os/user"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCurrentUser(t *testing.T) {
	t.Parallel()

	u, err := currentUser(func() (*user.User, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	}, 10*time.Millisecond)
	require.Nil(t, u)
	require.Error(t, err)
	require.True(t, trace.IsLimitExceeded(err))

	u, err = currentUser(func() (*user.User, error) {
		return nil, nil
	}, lookupTimeout)
	require.Nil(t, u)
	require.NoError(t, err)
}
