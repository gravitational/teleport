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
