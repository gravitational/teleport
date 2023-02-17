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

package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldDeleteServerHeartbeatsOnShutdown(t *testing.T) {
	type contextKey string
	ctx := context.Background()

	require.True(t, ShouldDeleteServerHeartbeatsOnShutdown(ctx))
	require.True(t, ShouldDeleteServerHeartbeatsOnShutdown(context.WithValue(ctx, contextKey("key"), "value")))
	require.False(t, ShouldDeleteServerHeartbeatsOnShutdown(ProcessReloadContext(ctx)))
	require.False(t, ShouldDeleteServerHeartbeatsOnShutdown(ProcessForkedContext(ctx)))
	require.False(t, ShouldDeleteServerHeartbeatsOnShutdown(ProcessReloadContext(ProcessForkedContext(ctx))))
	require.False(t, ShouldDeleteServerHeartbeatsOnShutdown(context.WithValue(ProcessReloadContext(ctx), contextKey("key"), "value")))
}
