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

package linux_test

import (
	"fmt"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/linux"
)

type permErrorFS struct {
	fstest.MapFS

	PermissionDeniedFiles []string
}

func (p *permErrorFS) Open(name string) (fs.File, error) {
	if slices.Contains(p.PermissionDeniedFiles, name) {
		return nil, fmt.Errorf("open %s: %w", name, fs.ErrPermission)
	}
	return p.MapFS.Open(name)
}

func TestDMI(t *testing.T) {
	successFS := fstest.MapFS{
		"product_name": &fstest.MapFile{
			Data: []byte("21J50013US\n"),
		},
		"product_serial": &fstest.MapFile{
			Data: []byte("PF0A0AAA\n"),
		},
		"board_serial": &fstest.MapFile{
			Data: []byte("L1AA00A00A0\n"),
		},
		"chassis_asset_tag": &fstest.MapFile{
			Data: []byte("No Asset Information\n"),
		},
	}

	realisticFS := permErrorFS{
		MapFS: successFS,
		PermissionDeniedFiles: []string{
			// Only root can read those in various Linux system.
			"product_serial",
			"board_serial",
		},
	}

	tests := []struct {
		name    string
		fs      fs.FS
		wantDMI *linux.DMIInfo
		wantErr string
	}{
		{
			name: "success",
			fs:   successFS,
			wantDMI: &linux.DMIInfo{
				ProductName:     "21J50013US",
				ProductSerial:   "PF0A0AAA",
				BoardSerial:     "L1AA00A00A0",
				ChassisAssetTag: "No Asset Information",
			},
		},
		{
			name: "realistic",
			fs:   &realisticFS,
			wantDMI: &linux.DMIInfo{
				ProductName:     "21J50013US",
				ChassisAssetTag: "No Asset Information",
			},
			wantErr: "permission denied",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := linux.DMIInfoFromFS(test.fs)
			if test.wantErr == "" {
				assert.NoError(t, err, "DMIInfoFromFS")
			} else {
				assert.ErrorContains(t, err, test.wantErr, "DMIInfoFromFS error mismatch")
			}

			if diff := cmp.Diff(test.wantDMI, got); diff != "" {
				t.Errorf("DMIInfoFromFS mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
