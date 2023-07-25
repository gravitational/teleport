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

	tmpl := templateSSHHostCert{
		principals: []string{"foo.example.com"},
	}

	dest := &DestinationMemory{}
	require.NoError(t, dest.CheckAndSetDefaults())

	ident := getTestIdent(t, "bot-test")
	err = tmpl.render(context.Background(), mockBot, ident, dest)
	require.NoError(t, err)

	// Make sure a cert is written. We just use a dummy cert (the CA fixture)
	certBytes, err := dest.Read(defaultSSHHostCertPrefix + sshHostCertSuffix)
	require.NoError(t, err)

	require.Equal(t, fixtures.SSHCAPublicKey, string(certBytes))

	// Make sure a CA is written.
	caBytes, err := dest.Read(defaultSSHHostCertPrefix + sshHostUserCASuffix)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(caBytes), fixtures.SSHCAPublicKey))
}
