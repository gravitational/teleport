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

// defaultMongoPrefix is the default prefix in generated MongoDB certs.
const defaultMongoPrefix = "mongo"

// templateMongo is a config template that generates TLS certs formatted for
// use with MongoDB.
type templateMongo struct{}

func (t *templateMongo) name() string {
	return TemplateMongoName
}

func (t *templateMongo) describe() []FileDescription {
	return []FileDescription{
		{
			Name: defaultMongoPrefix + ".crt",
		},
		{
			Name: defaultMongoPrefix + ".cas",
		},
	}
}

func (t *templateMongo) render(
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
		OutputPath: defaultMongoPrefix,
		Writer: &BotConfigWriter{
			dest: destination,
		},
		Key:    key,
		Format: identityfile.FormatMongo,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("Wrote MongoDB identity files: %+v", files)

	return nil
}
