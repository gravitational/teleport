/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package sshutils

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
)

type mockErrorWithExitStatus struct {
}

func (e mockErrorWithExitStatus) ExitStatus() int {
	return 2
}
func (e mockErrorWithExitStatus) Error() string {
	return "mockErrorWithExitStatus"
}

type mockExecExitError struct {
	sys any
}

func (e mockExecExitError) Sys() any {
	return e.sys
}
func (e mockExecExitError) Error() string {
	return "mockExecExitError"
}

func TestExitCodeFromExecError(t *testing.T) {
	// These struct types cannot be mocked. Implementation uses interfaces
	// instead of these types. Double check if these types satisfy the
	// interfaces.
	require.ErrorAs(t, &ssh.ExitError{}, new(errorWithExitStatus))
	require.ErrorAs(t, &exec.ExitError{}, new(execExitError))

	tests := []struct {
		name  string
		input error
		want  int
	}{
		{
			name:  "success",
			input: nil,
			want:  teleport.RemoteCommandSuccess,
		},
		{
			name:  "exec exit error",
			input: mockExecExitError{sys: syscall.WaitStatus(1 << 8)},
			want:  1,
		},
		{
			name:  "exec exit error with unknown sys",
			input: mockExecExitError{sys: "unknown"},
			want:  teleport.RemoteCommandFailure,
		},
		{
			name:  "ssh exit error",
			input: mockErrorWithExitStatus{},
			want:  2,
		},
		{
			name:  "unknown error",
			input: errors.New("unknown error"),
			want:  teleport.RemoteCommandFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ExitCodeFromExecError(tt.input))
		})
	}
}
