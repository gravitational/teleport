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

package tbot

import (
	"bytes"
	"context"
	"testing"

	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/destination"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
)

// Note: This test lives in main to avoid otherwise inevitable import cycles
// if we tried importing renewal code from the config package.

// validateTemplate loads and validates a config template from the destination
func validateTemplate(t *testing.T, tplI config.Template, dest destination.Destination) {
	t.Helper()

	// First, make sure all advertised files exist.
	for _, file := range tplI.Describe(dest) {
		// Don't bother checking directories, they're meant to be black
		// boxes. We could implement type-specific checks if we really
		// wanted.
		if file.IsDir {
			continue
		}

		bytes, err := dest.Read(file.Name)
		require.NoError(t, err)

		// Should at least be non-empty.
		t.Logf("Expected file %q for template %q has length: %d", file.Name, tplI.Name(), len(bytes))
		require.Truef(t, len(bytes) > 0, "file %q in template %q must be non-empty", file.Name, tplI.Name())
	}

	// Next, for supported template types, make sure they're valid.
	// TODO: consider adding further type-specific tests.
	switch tpl := tplI.(type) {
	case *config.TemplateIdentity:
		// Make sure the identityfile package can read this identity file.
		b, err := dest.Read(tpl.FileName)
		require.NoError(t, err)

		buf := bytes.NewBuffer(b)
		_, err = identityfile.Read(buf)
		require.NoError(t, err)
	case *config.TemplateTLSCAs:
		b, err := dest.Read(tpl.HostCAPath)
		require.NoError(t, err)
		_, err = tlsca.ParseCertificatePEM(b)
		require.NoError(t, err)

		b, err = dest.Read(tpl.UserCAPath)
		require.NoError(t, err)
		_, err = tlsca.ParseCertificatePEM(b)
		require.NoError(t, err)
	}
}

// TestTemplateRendering performs a full renewal and ensures all expected
// default config templates are present.
func TestDefaultTemplateRendering(t *testing.T) {
	t.Parallel()

	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	_ = testhelpers.MakeAndRunTestAuthServer(t, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, fc)

	// Make and join a new bot instance.
	botParams := testhelpers.MakeBot(t, rootClient, "test")
	botConfig := testhelpers.MakeMemoryBotConfig(t, fc, botParams)
	storage, err := botConfig.Storage.GetDestination()
	require.NoError(t, err)
	b := New(botConfig, utils.NewLoggerForTests(), nil)

	ident, err := b.getIdentityFromToken()
	require.NoError(t, err)
	botClient := testhelpers.MakeBotAuthClient(t, fc, ident)
	b._ident = ident
	b._client = botClient

	err = b.renew(context.Background(), storage)
	require.NoError(t, err)

	dest := botConfig.Destinations[0]
	destImpl, err := dest.GetDestination()
	require.NoError(t, err)

	for _, templateName := range config.GetRequiredConfigs() {
		cfg := dest.GetConfigByName(templateName)
		require.NotNilf(t, cfg, "template %q must exist", templateName)

		validateTemplate(t, cfg, destImpl)
	}
}
