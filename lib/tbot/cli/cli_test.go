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

package cli

import (
	"fmt"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

type configMutatorMock struct {
	mock.Mock
}

func (m *configMutatorMock) action(mut ConfigMutator) error {
	args := m.Called(mut)
	return args.Error(0)
}

type genericExecutorMock[T any] struct {
	mock.Mock
}

func (m *genericExecutorMock[T]) action(cmd *T) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func buildMinimalKingpinApp(subcommandName string) (app *kingpin.Application, subcommand *kingpin.CmdClause) {
	app = utils.InitCLIParser("tbot", "test").Interspersed(false)
	subcommand = app.Command(subcommandName, "subcommand")

	return
}

// TestConfigMutators that all config mutator-style match on their expected
// CLI args and return appropriately-typed parse results. This does not validate
// that the resulting configuration is valid (i.e. may not successfully pass
// conversion to BotConfig and the associated CheckAndSetDefaults())
func TestConfigMutators(t *testing.T) {
	tests := []struct {
		name         string
		args         [][]string
		buildCommand func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner
		assert       func(t *testing.T, value any)
	}{
		{
			name: "legacy",
			args: [][]string{{}, {"legacy"}},
			buildCommand: func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner {
				return NewLegacyCommand(parent, callback)
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &LegacyCommand{}, value)
			},
		},
		{
			name: "identity",
			args: [][]string{
				{"identity", "--destination=foo"},
				{"id", "--destination=foo"},
				{"ssh", "--destination=foo"},
			},
			buildCommand: func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner {
				return NewIdentityCommand(parent, callback)
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &IdentityCommand{}, value)
			},
		},

		{
			name: "database",
			args: [][]string{
				{"database", "--destination=foo", "--service=foo", "--username=bar", "--database=baz"},
				{"db", "--destination=foo", "--service=foo", "--username=bar", "--database=baz"},
			},
			buildCommand: func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner {
				return NewDatabaseCommand(parent, callback)
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &DatabaseCommand{}, value)
			},
		},
		{
			name: "kubernetes",
			args: [][]string{
				{"kubernetes", "--destination=foo", "--kubernetes-cluster=foo"},
				{"k8s", "--destination=foo", "--kubernetes-cluster=foo"},
			},
			buildCommand: func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner {
				return NewKubernetesCommand(parent, callback)
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &KubernetesCommand{}, value)
			},
		},
		{
			name: "application",
			args: [][]string{
				{"application", "--destination=foo", "--app=foo"},
				{"app", "--destination=foo", "--app=foo"},
			},
			buildCommand: func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner {
				return NewApplicationCommand(parent, callback)
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &ApplicationCommand{}, value)
			},
		},
		{
			name: "spiffe-x509-svid",
			args: [][]string{
				{"spiffe-x509-svid", "--destination=foo", "--svid-path=/bar"},
			},
			buildCommand: func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner {
				return NewSPIFFEX509SVIDCommand(parent, callback)
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &SPIFFEX509SVIDCommand{}, value)
			},
		},
		{
			name: "application-tunnel",
			args: [][]string{
				{"application-tunnel", "--app=foo", "--listen=tcp://0.0.0.0:8080"},
				{"app-tunnel", "--app=foo", "--listen=tcp://0.0.0.0:8080"},
			},
			buildCommand: func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner {
				return NewApplicationTunnelCommand(parent, callback)
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &ApplicationTunnelCommand{}, value)
			},
		},
		{
			name: "database-tunnel",
			args: [][]string{
				{"database-tunnel", "--service=foo", "--username=bar", "--database=baz", "--listen=tcp://0.0.0.0:8080"},
				{"db-tunnel", "--service=foo", "--username=bar", "--database=baz", "--listen=tcp://0.0.0.0:8080"},
			},
			buildCommand: func(parent *kingpin.CmdClause, callback MutatorAction) CommandRunner {
				return NewDatabaseTunnelCommand(parent, callback)
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &DatabaseTunnelCommand{}, value)
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			for i, argSet := range tt.args {
				argSet := argSet

				t.Run(fmt.Sprint(i), func(t *testing.T) {
					subcommandName := "sub"
					app, subcommand := buildMinimalKingpinApp(subcommandName)

					mockAction := configMutatorMock{}
					mockAction.On("action", mock.Anything).Return(nil)

					runner := tt.buildCommand(subcommand, mockAction.action)

					command, err := app.Parse(append([]string{subcommandName}, argSet...))
					require.NoError(t, err)

					match, err := runner.TryRun(command)
					require.NoError(t, err)
					require.True(t, match)

					mockAction.AssertCalled(t, "action", mock.Anything)

					arg := mockAction.Calls[0].Arguments.Get(0)
					tt.assert(t, arg)
				})
			}
		})
	}
}

func TestExecutors(t *testing.T) {
	// Note: Currently all executor-style
	tests := []struct {
		name         string
		args         []string
		buildCommand func(app *kingpin.Application) (CommandRunner, *mock.Mock)
		assert       func(t *testing.T, value any)
	}{
		{
			name: "init",
			args: []string{"init"},
			buildCommand: func(app *kingpin.Application) (CommandRunner, *mock.Mock) {
				m := &genericExecutorMock[InitCommand]{}
				return NewInitCommand(app, m.action), &m.Mock
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &InitCommand{}, value)
			},
		},
		{
			name: "db",
			args: []string{"db"},
			buildCommand: func(app *kingpin.Application) (CommandRunner, *mock.Mock) {
				m := &genericExecutorMock[DBCommand]{}
				return NewDBCommand(app, m.action), &m.Mock
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &DBCommand{}, value)
			},
		},
		{
			// note: this expects to be mounted to a "kube" parent command. for
			// the test, we'll just mount it to the application.
			name: "kube credentials",
			args: []string{"credentials", "--destination-dir=foo"},
			buildCommand: func(app *kingpin.Application) (CommandRunner, *mock.Mock) {
				m := &genericExecutorMock[KubeCredentialsCommand]{}
				return NewKubeCredentialsCommand(app, m.action), &m.Mock
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &KubeCredentialsCommand{}, value)
			},
		},
		{
			name: "migrate",
			args: []string{"migrate"},
			buildCommand: func(app *kingpin.Application) (CommandRunner, *mock.Mock) {
				m := &genericExecutorMock[MigrateCommand]{}
				return NewMigrateCommand(app, m.action), &m.Mock
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &MigrateCommand{}, value)
			},
		},
		{
			name: "proxy",
			args: []string{"proxy"},
			buildCommand: func(app *kingpin.Application) (CommandRunner, *mock.Mock) {
				m := &genericExecutorMock[ProxyCommand]{}
				return NewProxyCommand(app, m.action), &m.Mock
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &ProxyCommand{}, value)
			},
		},
		{
			name: "ssh-multiplexer-proxy-command",
			args: []string{"ssh-multiplexer-proxy-command", "/foo", "bar"},
			buildCommand: func(app *kingpin.Application) (CommandRunner, *mock.Mock) {
				m := &genericExecutorMock[SSHMultiplerProxyCommand]{}
				return NewSSHMultiplexerProxyCommand(app, m.action), &m.Mock
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &SSHMultiplerProxyCommand{}, value)
			},
		},
		{
			name: "ssh-proxy-command",
			args: []string{"ssh-proxy-command", "--user=foo", "--host=bar", "--proxy-server=baz", "--tls-routing", "--connection-upgrade"},
			buildCommand: func(app *kingpin.Application) (CommandRunner, *mock.Mock) {
				m := &genericExecutorMock[SSHProxyCommand]{}
				return NewSSHProxyCommand(app, m.action), &m.Mock
			},
			assert: func(t *testing.T, value any) {
				require.IsType(t, &SSHProxyCommand{}, value)
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			app, _ := buildMinimalKingpinApp("sub")

			runner, mockAction := tt.buildCommand(app)
			mockAction.On("action", mock.Anything).Return(nil)

			command, err := app.Parse(tt.args)
			require.NoError(t, err)

			match, err := runner.TryRun(command)
			require.NoError(t, err)
			require.True(t, match)

			mockAction.AssertCalled(t, "action", mock.Anything)

			arg := mockAction.Calls[0].Arguments.Get(0)
			tt.assert(t, arg)
		})
	}
}
