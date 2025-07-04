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

package provisioning

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func withErrCheck(fn func(*testing.T, error)) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, msgAndArgs ...any) {
		tt := t.(*testing.T)
		tt.Helper()
		fn(tt, err)
	}
}

var badParameterCheck = withErrCheck(func(t *testing.T, err error) {
	t.Helper()
	require.True(t, trace.IsBadParameter(err), `expected "bad parameter", but got %v`, err)
})

func TestOperationCheckAndSetDefaults(t *testing.T) {
	stubAction := Action{}
	baseCfg := OperationConfig{
		Name:        "name",
		Actions:     []Action{stubAction},
		AutoConfirm: true,
		Output:      os.Stdout,
	}
	tests := []struct {
		desc            string
		getConfig       func() OperationConfig
		want            OperationConfig
		wantErrContains string
	}{
		{
			desc:      "valid config",
			getConfig: func() OperationConfig { return baseCfg },
			want:      baseCfg,
		},
		{
			desc: "defaults output to stdout",
			getConfig: func() OperationConfig {
				cfg := baseCfg
				cfg.Output = nil
				return cfg
			},
			want: baseCfg,
		},
		{
			desc: "trims trailing and leading spaces",
			getConfig: func() OperationConfig {
				cfg := baseCfg
				cfg.Name = "\t\n " + cfg.Name + "\t\n "
				return cfg
			},
			want: baseCfg,
		},
		{
			desc: "missing name is an error",
			getConfig: func() OperationConfig {
				cfg := baseCfg
				cfg.Name = ""
				return cfg
			},
			wantErrContains: "missing operation name",
		},
		{
			desc: "spaces in name is an error",
			getConfig: func() OperationConfig {
				cfg := baseCfg
				cfg.Name = "some thing"
				return cfg
			},
			wantErrContains: "does not match regex",
		},
		{
			desc: "missing actions is an error",
			getConfig: func() OperationConfig {
				cfg := baseCfg
				cfg.Actions = nil
				return cfg
			},
			wantErrContains: "missing operation actions",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := test.getConfig()
			err := got.checkAndSetDefaults()
			if test.wantErrContains != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErrContains)
				return
			}
			require.Empty(t, cmp.Diff(test.want, got,
				cmp.AllowUnexported(Action{}),
				cmpopts.IgnoreFields(OperationConfig{}, "Output"),
			))
			require.NotNil(t, got.Output)
		})
	}
}

func TestNewAction(t *testing.T) {
	noOpRunnerFn := func(context.Context) error { return nil }
	baseCfg := ActionConfig{
		Name:     "name",
		Summary:  "summary",
		Details:  "details",
		RunnerFn: noOpRunnerFn,
	}
	tests := []struct {
		desc      string
		getConfig func() ActionConfig
		errCheck  require.ErrorAssertionFunc
		want      Action
	}{
		{
			desc:      "valid config",
			getConfig: func() ActionConfig { return baseCfg },
			errCheck:  require.NoError,
			want:      Action{config: baseCfg},
		},
		{
			desc: "trims trailing and leading spaces",
			getConfig: func() ActionConfig {
				cfg := baseCfg
				cfg.Name = "\t\n " + cfg.Name + "\t\n "
				return cfg
			},
			errCheck: require.NoError,
			want:     Action{config: baseCfg},
		},
		{
			desc: "missing name is an error",
			getConfig: func() ActionConfig {
				cfg := baseCfg
				cfg.Name = ""
				return cfg
			},
			errCheck: badParameterCheck,
		},
		{
			desc: "spaces in name is an error",
			getConfig: func() ActionConfig {
				cfg := baseCfg
				cfg.Name = "some thing"
				return cfg
			},
			errCheck: badParameterCheck,
		},
		{
			desc: "missing summary is an error",
			getConfig: func() ActionConfig {
				cfg := baseCfg
				cfg.Summary = ""
				return cfg
			},
			errCheck: badParameterCheck,
		},
		{
			desc: "missing details is an error",
			getConfig: func() ActionConfig {
				cfg := baseCfg
				cfg.Details = ""
				return cfg
			},
			errCheck: badParameterCheck,
		},
		{
			desc: "missing runner is an error",
			getConfig: func() ActionConfig {
				cfg := baseCfg
				cfg.RunnerFn = nil
				return cfg
			},
			errCheck: badParameterCheck,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			_, err := NewAction(test.getConfig())
			test.errCheck(t, err)
		})
	}
}

func TestRun(t *testing.T) {
	actionA := Action{
		config: ActionConfig{
			Name:     "actionA",
			Summary:  "actionA summary",
			Details:  "actionA details",
			RunnerFn: func(context.Context) error { return nil },
		},
	}
	actionB := Action{
		config: ActionConfig{
			Name:     "actionB",
			Summary:  "actionB summary",
			Details:  "actionB details",
			RunnerFn: func(context.Context) error { return nil },
		},
	}
	failingAction := Action{
		config: ActionConfig{
			Name:     "actionC",
			Summary:  "actionC summary",
			Details:  "actionC details",
			RunnerFn: func(context.Context) error { return trace.AccessDenied("access denied") },
		},
	}

	tests := []struct {
		desc     string
		config   OperationConfig
		errCheck require.ErrorAssertionFunc
	}{
		{
			desc: "success",
			config: OperationConfig{
				Name: "op-name",
				Actions: []Action{
					actionA,
					actionB,
				},
				AutoConfirm: true,
			},
			errCheck: require.NoError,
		},
		{
			desc: "failed action does not enumerate the failed step",
			config: OperationConfig{
				Name: "op-name",
				Actions: []Action{
					failingAction,
				},
				AutoConfirm: true,
			},
			errCheck: withErrCheck(func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
				// the error should indicate the failed action
				require.Contains(t, err.Error(), `"actionC" failed`)
				// but it should not number the step, since there's only
				// one action.
				require.NotContains(t, err.Error(), `1`)
			}),
		},
		{
			desc: "failed action fails the entire operation",
			config: OperationConfig{
				Name: "op-name",
				Actions: []Action{
					actionA,
					failingAction,
					actionB,
				},
				AutoConfirm: true,
			},
			errCheck: withErrCheck(func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
				// error should indicate step number and action name that failed
				require.Contains(t, err.Error(), `step 2 "actionC" failed`)
			}),
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()
			err := Run(ctx, test.config)
			test.errCheck(t, err)
		})
	}
}

func TestWriteOperationPlan(t *testing.T) {
	actionA := Action{
		config: ActionConfig{
			Name:     "actionA",
			Summary:  "<actionA summary>",
			Details:  "<actionA details>",
			RunnerFn: func(context.Context) error { return trace.NotImplemented("not implemented") },
		},
	}
	actionB := Action{
		config: ActionConfig{
			Name:     "actionB",
			Summary:  "<actionB summary>",
			Details:  "<actionB details>",
			RunnerFn: func(context.Context) error { return trace.NotImplemented("not implemented") },
		},
	}
	tests := []struct {
		desc   string
		config OperationConfig
		want   string
	}{
		{
			desc: "operation with only one action does not enumerate the step",
			config: OperationConfig{
				Name:        "op-name",
				Actions:     []Action{actionA},
				AutoConfirm: true,
			},
			want: strings.TrimLeft(`
"op-name" will perform the following action:

<actionA summary>.
actionA: <actionA details>

`, "\n"),
		},
		{
			desc: "operation with multiple actions enumerates the steps",
			config: OperationConfig{
				Name:        "op-name",
				Actions:     []Action{actionA, actionB},
				AutoConfirm: true,
			},
			want: strings.TrimLeft(`
"op-name" will perform the following actions:

1. <actionA summary>.
actionA: <actionA details>

2. <actionB summary>.
actionB: <actionB details>

`, "\n"),
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var got bytes.Buffer
			test.config.Output = &got
			require.NoError(t, writeOperationPlan(test.config))
			require.Empty(t, cmp.Diff(test.want, got.String()))
		})
	}
}
