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

package protocol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestWriteCmd(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		expected []byte
		wantErr  bool
	}{
		{
			name:     "string",
			val:      "test val",
			expected: []byte("$8\r\ntest val\r\n"),
		},
		{
			name:     "int",
			val:      1,
			expected: []byte(":1\r\n"),
		},
		{
			name:     "int32",
			val:      19,
			expected: []byte(":19\r\n"),
		},
		{
			name:     "int64",
			val:      64,
			expected: []byte(":64\r\n"),
		},
		{
			name:     "float",
			val:      3.14,
			expected: []byte("$4\r\n3.14\r\n"),
		},
		{
			name:     "[]string",
			val:      []string{"test val1", "test val 2"},
			expected: []byte("*2\r\n$9\r\ntest val1\r\n$10\r\ntest val 2\r\n"),
		},
		{
			name:     "[]nil",
			val:      []any{nil},
			expected: []byte("*1\r\n$-1\r\n"),
		},
		{
			name:     "[]bool",
			val:      []bool{true, false},
			expected: []byte("*2\r\n$1\r\n1\r\n$1\r\n0\r\n"),
		},
		{
			name:     "error",
			val:      errors.New("something bad"),
			expected: []byte("-ERR something bad\r\n"),
		},
		{
			name:     "multi-line error",
			val:      errors.New("something bad.\r\n  \n  and another line"),
			expected: []byte("-ERR something bad. and another line\r\n"),
		},
		{
			name:     "Teleport error",
			val:      trace.Errorf("something bad"),
			expected: []byte("-ERR Teleport: something bad\r\n"),
		},
		{
			name:     "Redis nil",
			val:      redis.Nil,
			expected: []byte("$-1\r\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			wr := redis.NewWriter(buf)

			if err := WriteCmd(wr, tt.val); (err != nil) != tt.wantErr {
				t.Errorf("WriteCmd() error = %v, wantErr %v", err, tt.wantErr)
			}

			require.Equal(t, tt.expected, buf.Bytes())
		})
	}
}

func TestReadWriteStatus(t *testing.T) {
	inputStatusBytes := []byte("+status\r\n")

	// Read the status into a redis.Cmd.
	cmd := &redis.Cmd{}
	err := cmd.ReadReply(redis.NewReader(bytes.NewReader(inputStatusBytes)))
	require.NoError(t, err)

	// Verify result.
	value, err := cmd.Result()
	require.NoError(t, err)
	require.Equal(t, "status", fmt.Sprintf("%v", value))

	// Verify WriteCmd.
	outputStatusBytes := &bytes.Buffer{}
	err = WriteCmd(redis.NewWriter(outputStatusBytes), value)
	require.NoError(t, err)
	require.Equal(t, string(inputStatusBytes), outputStatusBytes.String())
}

func TestMakeUnknownCommandErrorForCmd(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name          string
		command       []any
		expectedError redis.RedisError
	}{
		{
			name:          "HELLO",
			command:       []any{"HELLO", 3, "AUTH", "user", "TOKEN"},
			expectedError: "ERR unknown command 'HELLO', with args beginning with: '3' 'AUTH' 'user' 'TOKEN'",
		},
		{
			name:          "no extra args",
			command:       []any{"abcdef"},
			expectedError: "ERR unknown command 'abcdef', with args beginning with: ",
		},
		{
			name:          "cluster",
			command:       []any{"cluster", "aaa", "bbb"},
			expectedError: "ERR unknown subcommand 'aaa'. Try CLUSTER HELP.",
		},
		{
			name:          "command",
			command:       []any{"command", "aaa", "bbb"},
			expectedError: "ERR unknown subcommand 'aaa'. Try COMMAND HELP.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := redis.NewCmd(ctx, test.command...)
			actualError := MakeUnknownCommandErrorForCmd(cmd)
			require.Equal(t, test.expectedError, actualError)
		})
	}
}
