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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

type fakeKubeGateway struct {
	gateway.Kube
}

func (m fakeKubeGateway) KubeconfigPath() string { return "test.kubeconfig" }

func TestNewKubeCLICommand(t *testing.T) {
	cmd, err := NewKubeCLICommand(fakeKubeGateway{})
	require.NoError(t, err)
	require.Equal(t, []string{"KUBECONFIG=test.kubeconfig"}, cmd.Exec.Env)
}
