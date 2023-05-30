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
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/fixtures"
)

func TestTemplateSSHHostCertRender(t *testing.T) {
	cfg, err := newTestConfig("example.com")
	require.NoError(t, err)

	mockBot := newMockProvider(cfg)

	template := TemplateSSHHostCert{
		Prefix:     "example",
		Principals: []string{"foo.example.com"},
	}
	require.NoError(t, template.CheckAndSetDefaults())

	memory := &DestinationMemory{}
	dest := &DestinationConfig{
		DestinationMixin: DestinationMixin{
			Memory: memory,
		},
	}
	require.NoError(t, dest.CheckAndSetDefaults())

	ident := getTestIdent(t, "bot-test")
	err = template.Render(context.Background(), mockBot, ident, dest)
	require.NoError(t, err)

	// Make sure a cert is written. We just use a dummy cert (the CA fixture)
	certBytes, err := memory.Read(template.Prefix + sshHostCertSuffix)
	require.NoError(t, err)

	require.Equal(t, fixtures.SSHCAPublicKey, string(certBytes))

	// Make sure a CA is written.
	caBytes, err := memory.Read(template.Prefix + sshHostUserCASuffix)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(caBytes), fixtures.SSHCAPublicKey))
}
