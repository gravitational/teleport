/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package sftputils

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckFileInfoName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		shouldFail bool
	}{
		{"", true},
		{".", true},
		{"..", true},
		{"foo", false},
		{"foo/", true},
		{"foo\\", true},
		{"foo/bar", true},
		{"foo\\bar", true},
		{"../foo", true},
		{"..\\foo", true},
		{"foo/../bar", true},
		{"foo\\..\\bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileInfo := &mockFileInfo{name: tt.name}
			err := CheckFileInfoName(fileInfo)
			if tt.shouldFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type mockFileInfo struct {
	name string
}

func (f *mockFileInfo) Name() string {
	return f.name
}

func (f *mockFileInfo) Size() int64 {
	return 0
}

func (f *mockFileInfo) Mode() os.FileMode {
	return 0
}

func (f *mockFileInfo) ModTime() time.Time {
	return time.Now()
}

func (f *mockFileInfo) IsDir() bool {
	return false
}

func (f *mockFileInfo) Sys() interface{} {
	return nil
}
