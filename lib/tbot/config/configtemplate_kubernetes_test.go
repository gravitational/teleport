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

package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/utils/golden"
)

// TestTemplateKubernetesRender renders a Kubernetes template and compares it
// to the saved golden result.
func TestTemplateKubernetesRender(t *testing.T) {
	dir := t.TempDir()

	cfg, err := newTestConfig("example.com")
	require.NoError(t, err)

	mockBot := newMockProvider(cfg)
	template := TemplateKubernetes{
		getExecutablePath: func() (string, error) {
			return "tbot", nil
		},
	}
	require.NoError(t, template.CheckAndSetDefaults())

	k8sCluster := "example"
	dest := &DestinationConfig{
		DestinationMixin: DestinationMixin{
			Directory: &DestinationDirectory{
				Path:     dir,
				Symlinks: botfs.SymlinksInsecure,
				ACLs:     botfs.ACLOff,
			},
		},
		KubernetesCluster: &KubernetesCluster{
			ClusterName: k8sCluster,
		},
	}

	ident := getTestIdent(t, "bot-test", kubernetesRequest(k8sCluster))

	err = template.Render(context.Background(), mockBot, ident, dest)
	require.NoError(t, err)

	kubeconfigBytes, err := os.ReadFile(filepath.Join(dir, template.Path))
	require.NoError(t, err)

	kubeconfigBytes = bytes.ReplaceAll(kubeconfigBytes, []byte(dir), []byte("/test/dir"))

	if golden.ShouldSet() {
		golden.SetNamed(t, "kubeconfig.yaml", kubeconfigBytes)
	}

	require.Equal(
		t, string(golden.GetNamed(t, "kubeconfig.yaml")), string(kubeconfigBytes),
	)
}
