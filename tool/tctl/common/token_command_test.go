/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
)

type addedToken struct {
	Token   string
	Roles   []string
	Expires time.Time
}

type listedToken struct {
	Kind     string
	Version  string
	Metadata struct {
		Name    string
		Expires time.Time
		ID      uint
	}
	Spec struct {
		Roles      []string
		JoinMethod string
	}
}

func TestTokens(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Apps: config.Apps{
			Service: config.Service{
				EnabledFlag: "true",
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))

	// Test all output formats of "tokens add".
	t.Run("add", func(t *testing.T) {
		buf, err := runTokensCommand(t, fileConfig, []string{"add", "--type=node"})
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(buf.String(), "The invite token:"))

		buf, err = runTokensCommand(t, fileConfig, []string{"add", "--type=node,app", "--format", teleport.Text})
		require.NoError(t, err)
		require.Equal(t, strings.Count(buf.String(), "\n"), 1)

		buf, err = runTokensCommand(t, fileConfig, []string{"add", "--type=node,app", "--format", teleport.JSON})
		require.NoError(t, err)
		out := mustDecodeJSON[addedToken](t, buf)

		require.Len(t, out.Roles, 2)
		require.Equal(t, types.KindNode, strings.ToLower(out.Roles[0]))
		require.Equal(t, types.KindApp, strings.ToLower(out.Roles[1]))

		buf, err = runTokensCommand(t, fileConfig, []string{"add", "--type=node,app", "--format", teleport.YAML})
		require.NoError(t, err)
		out = mustDecodeYAML[addedToken](t, buf)

		require.Len(t, out.Roles, 2)
		require.Equal(t, types.KindNode, strings.ToLower(out.Roles[0]))
		require.Equal(t, types.KindApp, strings.ToLower(out.Roles[1]))

		buf, err = runTokensCommand(t, fileConfig, []string{"add", "--type=kube"})
		require.NoError(t, err)
		require.Contains(t, buf.String(), `--set roles="kube\,app\,discovery"`,
			"Command print out should include setting kube, app and discovery roles for helm install.")
	})

	// Test all output formats of "tokens ls".
	t.Run("ls", func(t *testing.T) {
		buf, err := runTokensCommand(t, fileConfig, []string{"ls"})
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(buf.String(), "Token "))
		require.Equal(t, 7, strings.Count(buf.String(), "\n")) // account for header lines

		buf, err = runTokensCommand(t, fileConfig, []string{"ls", "--format", teleport.Text})
		require.NoError(t, err)
		require.Equal(t, 5, strings.Count(buf.String(), "\n"))

		buf, err = runTokensCommand(t, fileConfig, []string{"ls", "--format", teleport.JSON})
		require.NoError(t, err)
		jsonOut := mustDecodeJSON[[]listedToken](t, buf)
		require.Len(t, jsonOut, 5)

		buf, err = runTokensCommand(t, fileConfig, []string{"ls", "--format", teleport.YAML})
		require.NoError(t, err)
		yamlOut := mustDecodeYAML[[]listedToken](t, buf)
		require.Len(t, yamlOut, 5)

		require.Equal(t, jsonOut, yamlOut)
	})
}
