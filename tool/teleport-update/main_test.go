// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autoupdatelib "github.com/gravitational/teleport/lib/autoupdate"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
)

const initTestSentinel = "init_test"

func TestMain(m *testing.M) {
	if slices.Contains(os.Args, initTestSentinel) {
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func BenchmarkInit(b *testing.B) {
	executable, err := os.Executable()
	require.NoError(b, err)

	for b.Loop() {
		cmd := exec.Command(executable, initTestSentinel)
		err := cmd.Run()
		assert.NoError(b, err)
	}
}

func TestStatusExitCode(t *testing.T) {
	lowVersion := "1.2.3"
	highVersion := "1.2.4"
	tests := []struct {
		name             string
		ccfg             *cliConfig
		status           autoupdate.Status
		expectedExitCode int
	}{
		{
			name: "no --is-up-to-date passed, should update",
			ccfg: &cliConfig{StatusWithExitCode: false},
			status: autoupdate.Status{
				UpdateStatus: autoupdate.UpdateStatus{
					Active: autoupdate.Revision{
						Version: lowVersion,
					},
				},
				FindResp: autoupdate.FindResp{
					Target: autoupdate.Revision{
						Version: highVersion,
					},
					InWindow: true,
				},
			},
			expectedExitCode: 0,
		},
		{
			name: "--is-up-to-date passed, different version in maintenance",
			ccfg: &cliConfig{StatusWithExitCode: true},
			status: autoupdate.Status{
				UpdateStatus: autoupdate.UpdateStatus{
					Active: autoupdate.Revision{
						Version: lowVersion,
					},
				},
				FindResp: autoupdate.FindResp{
					Target: autoupdate.Revision{
						Version: highVersion,
					},
					InWindow: true,
				},
			},
			expectedExitCode: notUpToDateExitCode,
		},
		{
			name: "--is-up-to-date passed, different version out of maintenance",
			ccfg: &cliConfig{StatusWithExitCode: true},
			status: autoupdate.Status{
				UpdateStatus: autoupdate.UpdateStatus{
					Active: autoupdate.Revision{
						Version: lowVersion,
					},
				},
				FindResp: autoupdate.FindResp{
					Target: autoupdate.Revision{
						Version: highVersion,
					},
					InWindow: false,
				},
			},
			expectedExitCode: 0,
		},
		{
			name: "--is-up-to-date passed, same version in maintenance",
			ccfg: &cliConfig{StatusWithExitCode: true},
			status: autoupdate.Status{
				UpdateStatus: autoupdate.UpdateStatus{
					Active: autoupdate.Revision{
						Version: highVersion,
					},
				},
				FindResp: autoupdate.FindResp{
					Target: autoupdate.Revision{
						Version: highVersion,
					},
					InWindow: true,
				},
			},
			expectedExitCode: 0,
		},
		{
			name: "--is-up-to-date passed, same version in maintenance, edition mismatch",
			ccfg: &cliConfig{StatusWithExitCode: true},
			status: autoupdate.Status{
				UpdateStatus: autoupdate.UpdateStatus{
					Active: autoupdate.Revision{
						Version: highVersion,
					},
				},
				FindResp: autoupdate.FindResp{
					Target: autoupdate.Revision{
						Version: highVersion,
						Flags:   autoupdatelib.FlagEnterprise,
					},
					InWindow: true,
				},
			},
			expectedExitCode: notUpToDateExitCode,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expectedExitCode, statusExitCode(tt.ccfg, tt.status))
		})
	}
}
