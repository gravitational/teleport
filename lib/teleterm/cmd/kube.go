/*
Copyright 2023 Gravitational, Inc.

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

package cmd

import (
	"fmt"
	"os/exec"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

// KubeCLICommandProvider provides CLI commands for kube gateways.
type KubeCLICommandProvider struct {
}

func NewKubeCLICommandProvider() KubeCLICommandProvider {
	return KubeCLICommandProvider{}
}

func (p KubeCLICommandProvider) GetCommand(gateway gateway.GatewayReader) (*exec.Cmd, error) {
	// Use kubectl version as placeholders. Only env should be used.
	cmd := exec.Command("kubectl", "version")
	cmd.Env = []string{fmt.Sprintf("%v=%v", teleport.EnvKubeConfig, gateway.KubeconfigPath())}
	return cmd, nil
}
