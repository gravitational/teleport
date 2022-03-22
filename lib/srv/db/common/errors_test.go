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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIAMAuthError(t *testing.T) {
	err := NewIAMAuthError("I am an %s error", "IAM auth")
	require.Equal(t, "I am an IAM auth error", err.Error())
	require.True(t, trace.IsAccessDenied(err))
	require.True(t, IsIAMAuthError(err))
	require.ErrorIs(t, err, NewIAMAuthError("I am an IAM auth error"))
	_, traceWrapped := err.(trace.DebugReporter)
	require.True(t, traceWrapped)
}
