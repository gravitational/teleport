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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
)

type mockHostCertAuth struct {
	mockAuth
}

func (m *mockHostCertAuth) GenerateHostCert(
	ctx context.Context,
	key []byte, hostID, nodeName string, principals []string,
	clusterName string, role types.SystemRole, ttl time.Duration,
) ([]byte, error) {
	// We could generate a cert easily enough here, but the template generates a
	// random key each run so the resulting cert will change too.
	// The CA fixture isn't even a cert but we never examine it, so it'll do the
	// job.
	return []byte(fixtures.SSHCAPublicKey), nil
}

func TestTemplateSSHHostCertRender(t *testing.T) {
	mockAuth := &mockHostCertAuth{
		mockAuth: *newMockAuth(t),
	}

	cfg, err := NewDefaultConfig("example.com")
	require.NoError(t, err)

	mockBot := newMockBot(cfg, mockAuth)

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
	err = template.Render(context.Background(), mockBot, ident, ident, dest)
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
