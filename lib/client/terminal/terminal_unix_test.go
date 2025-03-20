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

package terminal

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// fixedDummyFmt is dummy formatter which ignores entry argument in Format() call and returns the same value, always.
type fixedDummyFmt struct {
	bytes []byte
	err   error
}

func (r fixedDummyFmt) Format(entry *logrus.Entry) ([]byte, error) {
	return r.bytes, r.err
}

func Test_addCRFormatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		output  string
		wantErr bool
	}{
		{
			name:   "no newlines",
			input:  "foo bar baz",
			output: "foo bar baz",
		},
		{
			name:   "single newline",
			input:  "foo bar baz\n",
			output: "foo bar baz\r\n",
		},
		{
			name:   "multiple newlines",
			input:  "foo\nbar\nbaz\n",
			output: "foo\r\nbar\r\nbaz\r\n",
		},
		{
			name:    "propagate error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseFmt := fixedDummyFmt{bytes: []byte(tt.input)}
			if tt.wantErr {
				baseFmt.err = fmt.Errorf("dummy")
			}

			r := newCRFormatter(baseFmt)
			actual, err := r.Format(nil)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.output, string(actual))
			}

		})
	}
}
