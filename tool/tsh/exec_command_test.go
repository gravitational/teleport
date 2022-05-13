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
	os_exec "os/exec"
	"path"
	"testing"

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
	mkshell := func(fn string) {
		// use existing `sh` binary for tests.
		shPath, err := os_exec.LookPath("sh")
		require.NoError(t, err)

		file, err := os.ReadFile(shPath)
		require.NoError(t, err)

		err = os.WriteFile(fn, file, 0777)
		require.NoError(t, err)
	}

	tmp := t.TempDir()

	tests := []struct {
		name string
		init func(t *testing.T) string
		want string
	}{
		{
			name: "return custom shell",
			init: func(t *testing.T) string {
				err := os.MkdirAll(path.Join(tmp, "custom"), 0777)
				require.NoError(t, err)

				dummyShell := path.Join(tmp, "custom", "dummysh")
				mkshell(dummyShell)

				return dummyShell
			},
			want: path.Join(tmp, "custom", "dummysh"),
		},

		{
			name: "return $SHELL",
			init: func(t *testing.T) string {
				err := os.MkdirAll(path.Join(tmp, "SHELL"), 0777)
				require.NoError(t, err)

				dummyShell := path.Join(tmp, "SHELL", "dummysh")
				mkshell(dummyShell)

				t.Setenv("SHELL", dummyShell)

				return ""
			},
			want: path.Join(tmp, "SHELL", "dummysh"),
		},

		{
			name: "return sh from PATH",
			init: func(t *testing.T) string {
				err := os.MkdirAll(path.Join(tmp, "PATH_SH"), 0777)
				require.NoError(t, err)

				dummyShell := path.Join(tmp, "PATH_SH", "sh")
				mkshell(dummyShell)

				// unset shell, set path
				t.Setenv("SHELL", "")
				t.Setenv("PATH", path.Join(tmp, "PATH_SH"))

				return ""
			},
			want: path.Join(tmp, "PATH_SH", "sh"),
		},

		{
			name: "return bash from PATH",
			init: func(t *testing.T) string {
				err := os.MkdirAll(path.Join(tmp, "PATH_BASH"), 0777)
				require.NoError(t, err)

				dummyShell := path.Join(tmp, "PATH_BASH", "bash")
				mkshell(dummyShell)

				// unset shell, set path
				t.Setenv("SHELL", "")
				t.Setenv("PATH", path.Join(tmp, "PATH_BASH"))

				return ""
			},
			want: path.Join(tmp, "PATH_BASH", "bash"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customShell := tt.init(t)
			shell, err := findShell(customShell)
			require.NoError(t, err)
			require.Equal(t, tt.want, shell)
		})
	}
}

func Test_newExecCommand(t *testing.T) {
	app := kingpin.New("my test app", "")
	exec := newExecCommand(app)
	require.NotNil(t, exec)
	require.Equal(t, exec.cmd, app.GetCommand("exec"))
}
