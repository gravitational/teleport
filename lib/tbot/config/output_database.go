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

package config

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const DatabaseOutputType = "database"

// DatabaseFormat specifies if any special behavior should be invoked when
// producing artifacts. This allows for databases/clients that require unique
// formats or paths to be used.
type DatabaseFormat string

const (
	// UnspecifiedDatabaseFormat is the unset value and the default. This
	// should work for most databases.
	UnspecifiedDatabaseFormat DatabaseFormat = ""
	// TLSDatabaseFormat is for databases that require specifically named
	// outputs: tls.key, tls.crt and tls.cas
	TLSDatabaseFormat DatabaseFormat = "tls"
	// MongoDatabaseFormat indicates credentials should be generated which
	// are compatible with MongoDB.
	// This outputs `mongo.crt` and `mongo.cas`.
	MongoDatabaseFormat DatabaseFormat = "mongo"
	// CockroachDatabaseFormat indicates credentials should be generated which
	// are compatible with CockroachDB.
	// This outputs `cockroach/node.key`, `cockroach/node.crt` and
	// `cockroach/ca.crt`.
	CockroachDatabaseFormat DatabaseFormat = "cockroach"
)

var databaseFormats = []DatabaseFormat{
	UnspecifiedDatabaseFormat,
	TLSDatabaseFormat,
	MongoDatabaseFormat,
	CockroachDatabaseFormat,
}

// DatabaseOutput produces credentials which can be used to connect to a
// database through teleport.
type DatabaseOutput struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	// Formats specifies if any special behavior should be invoked when
	// producing artifacts. An empty value is supported by most database,
	// but CockroachDB and MongoDB require this value to be set to
	// `mongo` and `cockroach` respectively.
	Format DatabaseFormat `yaml:"format,omitempty"`
	// Service is the service name of the Teleport database. Generally this is
	// the name of the Teleport resource. This field is required for all types
	// of database.
	Service string `yaml:"service"`
	// Database is the name of the database to request access to.
	Database string `yaml:"database,omitempty"`
	// Username is the database username to request access as.
	Username string `yaml:"username,omitempty"`
}

func (o *DatabaseOutput) templates() []template {
	templates := []template{
		&templateTLSCAs{},
		&templateIdentity{},
	}
	if o.Format == MongoDatabaseFormat {
		templates = append(templates, &templateMongo{})
	}
	if o.Format == CockroachDatabaseFormat {
		templates = append(templates, &templateCockroach{})
	}
	if o.Format == TLSDatabaseFormat {
		templates = append(templates, &templateTLS{
			caCertType: types.HostCA,
		})
	}
	return templates
}

func (o *DatabaseOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	if err := identity.SaveIdentity(ident, o.Destination, identity.DestinationKinds()...); err != nil {
		return trace.Wrap(err, "persisting identity")
	}

	for _, t := range o.templates() {
		if err := t.render(ctx, p, ident, o.Destination); err != nil {
			return trace.Wrap(err, "rendering template %s", t.name())
		}
	}

	return nil
}

func (o *DatabaseOutput) Init() error {
	subDirs, err := listSubdirectories(o.templates())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(o.Destination.Init(subDirs))
}

func (o *DatabaseOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}

	if o.Service == "" {
		return trace.BadParameter("service must not be empty")
	}

	if !slices.Contains(databaseFormats, o.Format) {
		return trace.BadParameter("unrecognized format (%s)", o.Format)
	}

	return nil
}

func (o *DatabaseOutput) GetDestination() bot.Destination {
	return o.Destination
}

func (o *DatabaseOutput) GetRoles() []string {
	return o.Roles
}

func (o *DatabaseOutput) Describe() []FileDescription {
	var fds []FileDescription
	for _, t := range o.templates() {
		fds = append(fds, t.describe()...)
	}

	return fds
}

func (o DatabaseOutput) MarshalYAML() (interface{}, error) {
	type raw DatabaseOutput
	return withTypeHeader(raw(o), DatabaseOutputType)
}

func (o *DatabaseOutput) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw DatabaseOutput
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *DatabaseOutput) String() string {
	return fmt.Sprintf("%s (%s)", DatabaseOutputType, o.GetDestination())
}
