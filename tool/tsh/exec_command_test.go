// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"path"
	"testing"

	"github.com/gravitational/trace"

	"github.com/gravitational/kingpin"
	"github.com/stretchr/testify/require"
)

func Test_execCommand_runCommand(t *testing.T) {
	type args struct {
		cf *CLIConf
	}

	tmp := t.TempDir()

	tests := []struct {
		name      string
		cmd       func(t *testing.T) *execCommand
		args      args
		wantErr   bool
		postCheck func(t *testing.T)
	}{
		{
			name: "no profile",
			cmd: func(t *testing.T) *execCommand {
				app := kingpin.New("my test app", "")
				return newExecCommand(app)
			},
			args:    args{cf: &CLIConf{}},
			wantErr: false,
		},
		{
			name: "exit code 0, no error",
			cmd: func(t *testing.T) *execCommand {
				app := kingpin.New("my test app", "")
				exec := newExecCommand(app)
				exec.arguments = []string{"-c", "exit 0"}
				exec.shell = "bash"
				return exec
			},
			args:    args{cf: &CLIConf{}},
			wantErr: false,
		},
		{
			name: "exit code 1, error",
			cmd: func(t *testing.T) *execCommand {
				app := kingpin.New("my test app", "")
				exec := newExecCommand(app)
				exec.arguments = []string{"-c", "exit 1"}
				exec.shell = "bash"
				return exec
			},
			args:    args{cf: &CLIConf{}},
			wantErr: true,
		},
		{
			name: "check environment",
			cmd: func(t *testing.T) *execCommand {
				app := kingpin.New("my test app", "")
				exec := newExecCommand(app)
				exec.arguments = []string{"-c", `
cd $0
echo $TSH_COMMAND >> tsh_command.env
echo $TSH >> tsh.env`, tmp}
				exec.shell = "bash"
				return exec
			},
			args: args{cf: &CLIConf{executablePath: "/bin/tsh"}},
			postCheck: func(t *testing.T) {
				checkFile := func(fn string, contents string) {
					bytes, err := os.ReadFile(path.Join(tmp, fn))
					require.NoError(t, err)
					require.Equal(t, contents, string(bytes))
				}

				checkFile("tsh_command.env", "/bin/bash\n")
				checkFile("tsh.env", "/bin/tsh\n")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := tt.cmd(t)
			err := exec.runCommand(tt.args.cf)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.postCheck != nil {
				tt.postCheck(t)
			}
		})
	}
}

func Test_findShell(t *testing.T) {
	tests := []struct {
		name           string
		customShell    string
		shellEnvVar    string
		binariesInPath map[string]string
		want           string
		wantError      bool
	}{
		{
			name:           "return custom shell",
			customShell:    "dummysh",
			binariesInPath: map[string]string{"dummysh": "/opt/dummysh"},
			want:           "/opt/dummysh",
		},

		{
			name:           "missing custom shell",
			customShell:    "dummysh",
			binariesInPath: map[string]string{},
			wantError:      true,
		},

		{
			name:           "return sh from PATH",
			binariesInPath: map[string]string{"sh": "/bin/sh"},
			want:           "/bin/sh",
		},

		{
			name: "return bash from PATH",
			binariesInPath: map[string]string{
				"sh":   "/bin/sh",
				"bash": "/bin/bash",
			},
			want: "/bin/bash",
		},

		{
			name:        "return $SHELL",
			shellEnvVar: "shellEnvVar",
			binariesInPath: map[string]string{
				"shellEnvVar": "/opt/shellEnvVar",
				"sh":          "/bin/sh",
				"bash":        "/bin/bash",
			},
			want: "/opt/shellEnvVar",
		},

		{
			name:           "nothing found, shell set",
			shellEnvVar:    "shellEnvVar",
			binariesInPath: map[string]string{},
			wantError:      true,
		},
		{
			name:           "nothing found, shell unset",
			binariesInPath: map[string]string{},
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shellEnvVar != "" {
				t.Setenv("SHELL", tt.shellEnvVar)
			}

			lookPath := func(file string) (string, error) {
				if fp, found := tt.binariesInPath[file]; found {
					return fp, nil
				}

				return "", trace.NotFound("not found: %v", file)
			}
			out, err := findShell(tt.customShell, lookPath)

			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, out)
			}
		})
	}
}

func Test_newExecCommand(t *testing.T) {
	app := kingpin.New("my test app", "")
	exec := newExecCommand(app)
	require.NotNil(t, exec)
	require.Equal(t, exec.cmd, app.GetCommand("exec"))
}
