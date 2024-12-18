/*
Copyright 2015-2023 Gravitational, Inc.

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

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
)

// StartCmdConfig is mostly to test that the TOML file parsing works as
// expected.
func TestStartCmdConfig(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	testCases := []struct {
		name string
		args []string

		want CLI
	}{
		{
			name: "standard",
			args: []string{"start", "--config", "testdata/config.toml"},
			want: CLI{
				Debug: false,
				Start: StartCmdConfig{
					FluentdConfig: FluentdConfig{
						FluentdURL:            "https://localhost:8888/test.log",
						FluentdSessionURL:     "https://localhost:8888/session",
						FluentdCert:           filepath.Join(wd, "testdata", "fake-file"),
						FluentdKey:            filepath.Join(wd, "testdata", "fake-file"),
						FluentdCA:             filepath.Join(wd, "testdata", "fake-file"),
						FluentdMaxConnections: 10,
					},
					TeleportConfig: TeleportConfig{
						TeleportAddr:            "localhost:3025",
						TeleportIdentityFile:    filepath.Join(wd, "testdata", "fake-file"),
						TeleportRefreshEnabled:  true,
						TeleportRefreshInterval: 2 * time.Minute,
					},
					IngestConfig: IngestConfig{
						StorageDir:          "./storage",
						BatchSize:           20,
						SkipEventTypes:      map[string]struct{}{},
						SkipSessionTypesRaw: []string{"print"},
						SkipSessionTypes: map[string]struct{}{
							"print": {},
						},
						Timeout:     10 * time.Second,
						Concurrency: 5,
						WindowSize:  24 * time.Hour,
					},
					LockConfig: LockConfig{
						LockFailedAttemptsCount: 3,
						LockPeriod:              time.Minute,
					},
				},
			},
		},
		{
			name: "standard with debug enabled flag",
			args: []string{"--debug", "start", "--config", "testdata/config.toml"},
			want: CLI{
				Debug: true,
				Start: StartCmdConfig{
					FluentdConfig: FluentdConfig{
						FluentdURL:            "https://localhost:8888/test.log",
						FluentdSessionURL:     "https://localhost:8888/session",
						FluentdCert:           filepath.Join(wd, "testdata", "fake-file"),
						FluentdKey:            filepath.Join(wd, "testdata", "fake-file"),
						FluentdCA:             filepath.Join(wd, "testdata", "fake-file"),
						FluentdMaxConnections: 10,
					},
					TeleportConfig: TeleportConfig{
						TeleportAddr:            "localhost:3025",
						TeleportIdentityFile:    filepath.Join(wd, "testdata", "fake-file"),
						TeleportRefreshEnabled:  true,
						TeleportRefreshInterval: 2 * time.Minute,
					},
					IngestConfig: IngestConfig{
						StorageDir:          "./storage",
						BatchSize:           20,
						SkipEventTypes:      map[string]struct{}{},
						SkipSessionTypesRaw: []string{"print"},
						SkipSessionTypes: map[string]struct{}{
							"print": {},
						},
						Timeout:     10 * time.Second,
						Concurrency: 5,
						WindowSize:  24 * time.Hour,
					},
					LockConfig: LockConfig{
						LockFailedAttemptsCount: 3,
						LockPeriod:              time.Minute,
					},
				},
			},
		},
		{
			name: "debug enabled",
			args: []string{"start", "--config", "testdata/config-debug.toml"},
			want: CLI{
				Debug: true,
				Start: StartCmdConfig{
					FluentdConfig: FluentdConfig{
						FluentdURL:            "https://localhost:8888/test.log",
						FluentdSessionURL:     "https://localhost:8888/session",
						FluentdCert:           filepath.Join(wd, "testdata", "fake-file"),
						FluentdKey:            filepath.Join(wd, "testdata", "fake-file"),
						FluentdCA:             filepath.Join(wd, "testdata", "fake-file"),
						FluentdMaxConnections: 10,
					},
					TeleportConfig: TeleportConfig{
						TeleportAddr:            "localhost:3025",
						TeleportIdentityFile:    filepath.Join(wd, "testdata", "fake-file"),
						TeleportRefreshEnabled:  true,
						TeleportRefreshInterval: 2 * time.Minute,
					},
					IngestConfig: IngestConfig{
						StorageDir:          "./storage",
						BatchSize:           20,
						SkipEventTypes:      map[string]struct{}{},
						SkipSessionTypesRaw: []string{"print"},
						SkipSessionTypes: map[string]struct{}{
							"print": {},
						},
						Timeout:     10 * time.Second,
						Concurrency: 5,
						WindowSize:  24 * time.Hour,
					},
					LockConfig: LockConfig{
						LockFailedAttemptsCount: 3,
						LockPeriod:              time.Minute,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cli := CLI{}
			parser, err := kong.New(
				&cli,
				kong.UsageOnError(),
				kong.Configuration(KongTOMLResolver),
				kong.Name(pluginName),
				kong.Description(pluginDescription),
			)
			require.NoError(t, err)
			_, err = parser.Parse(tc.args)
			require.NoError(t, err)
			// reset config file and configure values since we only want to verify Start and Debug fields
			cli.Config = ""
			cli.Configure = ConfigureCmdConfig{}
			require.Equal(t, tc.want, cli)
		})
	}
}
