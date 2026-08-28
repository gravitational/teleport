/**
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

package web

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *fetchRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: &fetchRequest{
				startOffset: 0,
				endOffset:   1 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "negative start offset",
			req: &fetchRequest{
				startOffset: -1 * time.Second,
				endOffset:   1 * time.Second,
			},
			wantErr: true,
			errMsg:  "invalid time range",
		},
		{
			name: "negative end offset",
			req: &fetchRequest{
				startOffset: 0,
				endOffset:   -1 * time.Second,
			},
			wantErr: true,
			errMsg:  "invalid time range",
		},
		{
			name: "end before start",
			req: &fetchRequest{
				startOffset: 1 * time.Second,
				endOffset:   500 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "invalid time range (1s, 500ms)",
		},
		{
			name: "range too large",
			req: &fetchRequest{
				startOffset: 0,
				endOffset:   11 * time.Minute,
			},
			wantErr: true,
			errMsg:  "time range too large",
		},
		{
			name: "max valid range",
			req: &fetchRequest{
				startOffset: 0,
				endOffset:   10 * time.Minute,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequest(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDecodeBinaryRequest(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *fetchRequest
		wantErr bool
	}{
		{
			name: "valid request with current screen flag false",
			data: createFetchRequest(1000, 2000, 42, false),
			want: &fetchRequest{
				requestType:          requestTypeFetch,
				startOffset:          time.Duration(1000) * time.Millisecond,
				endOffset:            time.Duration(2000) * time.Millisecond,
				requestID:            42,
				requestCurrentScreen: false,
			},
			wantErr: false,
		},
		{
			name: "valid request with current screen flag true",
			data: createFetchRequest(1000, 2000, 42, true),
			want: &fetchRequest{
				requestType:          requestTypeFetch,
				startOffset:          time.Duration(1000) * time.Millisecond,
				endOffset:            time.Duration(2000) * time.Millisecond,
				requestID:            42,
				requestCurrentScreen: true,
			},
			wantErr: false,
		},
		{
			name:    "request too short",
			data:    make([]byte, requestHeaderSize-1),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "request too long",
			data:    make([]byte, requestHeaderSize+1),
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeBinaryRequest(tt.data)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEncodeScreenEvent(t *testing.T) {
	state := vt10x.TerminalState{
		Cols: 80,
		Rows: 24,
	}
	cols := 80
	rows := 24
	cursor := vt10x.Cursor{X: 10, Y: 5}

	result := encodeScreenEvent(state, cols, rows, cursor)

	require.Greater(t, len(result), requestHeaderSize)
	require.Equal(t, byte(eventTypeScreen), result[0])
	require.Equal(t, uint32(cols), binary.BigEndian.Uint32(result[1:5]))
	require.Equal(t, uint32(rows), binary.BigEndian.Uint32(result[5:9]))
	require.Equal(t, uint32(cursor.X), binary.BigEndian.Uint32(result[9:13]))
	require.Equal(t, uint32(cursor.Y), binary.BigEndian.Uint32(result[13:17]))
}
