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

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_ssoDiagContext_writeToBackend(t *testing.T) {
	diag := &SSODiagContext{
		AuthKind:  types.KindSAML,
		RequestID: "123",
		Info:      types.SSODiagnosticInfo{},
	}

	callCount := 0

	diagFn := func(ctx context.Context, authKind string, authRequestID string, info types.SSODiagnosticInfo) error {
		callCount++
		require.Truef(t, info.TestFlow, "CreateSSODiagnosticInfo must not be called if info.TestFlow is false.")
		require.Equal(t, diag.AuthKind, authKind)
		require.Equal(t, diag.RequestID, authRequestID)
		require.Equal(t, diag.Info, info)
		return nil
	}
	diag.DiagService = SSODiagServiceFunc(diagFn)

	// with TestFlow: false, no call is made.
	diag.Info.TestFlow = false
	diag.WriteToBackend(context.Background())
	require.Equal(t, 0, callCount)

	// with TestFlow: true, a call is made.
	diag.Info.TestFlow = true
	diag.WriteToBackend(context.Background())
	require.Equal(t, 1, callCount)
}
