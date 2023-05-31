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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const defaultCockroachDirName = "cockroach"

// templateCockroach generates certificates for CockroachDB. These are standard
// TLS certs but have specific naming requirements. We write them to a
// subdirectory to ensure naming is clear.
type templateCockroach struct{}

func (t *templateCockroach) name() string {
	return TemplateCockroachName
}

func (t *templateCockroach) describe() []FileDescription {
	return []FileDescription{
		{
			Name:  defaultCockroachDirName,
			IsDir: true,
		},
	}
}

func (t *templateCockroach) render(
	ctx context.Context,
	bot provider,
	identity *identity.Identity,
	destination bot.Destination,
) error {
	dbCAs, err := bot.GetCertAuthorities(ctx, types.DatabaseCA)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := newClientKey(identity, dbCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	cfg := identityfile.WriteConfig{
		OutputPath: defaultCockroachDirName,
		Writer: &BotConfigWriter{
			dest:    destination,
			subpath: defaultCockroachDirName,
		},
		Key:    key,
		Format: identityfile.FormatCockroach,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("Wrote CockroachDB files: %+v", files)

	return nil
}
