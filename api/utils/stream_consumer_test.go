/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package utils

import (
	"errors"
	"io"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestConsumeStreamToErrorIfEOF(t *testing.T) {
	type args struct {
		sendErr error
		stream  ClientReceiver[string]
	}
	type testCase struct {
		name      string
		args      args
		assertErr require.ErrorAssertionFunc
	}
	tests := []testCase{
		{
			name: "send error is nil", /* this is a special case to avoid locks */
			args: args{
				sendErr: nil,
				stream:  &fakeClientReceiver[string]{},
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.NoError(t, err)
			},
		},
		{
			name: "send error is not EOF",
			args: args{
				sendErr: errors.New("fake send error"),
				stream:  &fakeClientReceiver[string]{},
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "fake send error")
			},
		},
		{
			name: "send error is EOF and stream returns err immediately",
			args: args{
				sendErr: trace.Wrap(io.EOF),
				stream:  &fakeClientReceiver[string]{numberOfCallsToErr: 1},
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "fake error")
			},
		},
		{
			name: "send error is EOF and stream returns err after 10 calls",
			args: args{
				sendErr: trace.Wrap(io.EOF),
				stream:  &fakeClientReceiver[string]{numberOfCallsToErr: 10},
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "fake error")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ConsumeStreamToErrorIfEOF(tt.args.sendErr, tt.args.stream)
			tt.assertErr(t, err)
		})
	}
}

type fakeClientReceiver[T any] struct {
	numberOfCallsToErr int
}

func (f *fakeClientReceiver[T]) Recv() (T, error) {
	var v T
	f.numberOfCallsToErr--
	if f.numberOfCallsToErr == 0 {
		return v, errors.New("fake error")
	}
	return v, nil
}
