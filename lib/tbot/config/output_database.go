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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const DatabaseOutputType = "database"

type DatabaseSubtype string

var (
	// UnspecifiedDatabaseSubtype is the unset value
	UnspecifiedDatabaseSubtype DatabaseSubtype = ""
	// StandardDatabaseSubtype works with most databases and is the default.
	StandardDatabaseSubtype DatabaseSubtype = "tls"
	// MongoDatabaseSubtype indicates credentials should be generated which
	// are compatible with MongoDB.
	MongoDatabaseSubtype DatabaseSubtype = "mongo"
	// CockroachDatabaseSubtype indicates credentials should be generated which
	// are compatible with CockroachDB.
	CockroachDatabaseSubtype DatabaseSubtype = "cockroach"

	databaseSubtypes = []DatabaseSubtype{
		StandardDatabaseSubtype,
		MongoDatabaseSubtype,
		CockroachDatabaseSubtype,
	}
)

// DatabaseOutput produces credentials which can be used to connect to a
// database through teleport.
type DatabaseOutput struct {
	Common OutputCommon `yaml:",inline"`
	// Subtype indicates the type of the database you are generating credentials
	// for. An empty value is supported by most database, but CockroachDB and
	// MongoDB require this value to be set to `mongo` and `cockroach`
	// respectively.
	Subtype DatabaseSubtype `yaml:"subtype,omitempty"`
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
	if o.Subtype == MongoDatabaseSubtype {
		templates = append(templates, &templateMongo{})
	}
	if o.Subtype == CockroachDatabaseSubtype {
		templates = append(templates, &templateCockroach{})
	}
	if o.Subtype == StandardDatabaseSubtype {
		templates = append(templates, &templateTLS{
			caCertType: types.HostCA,
		})
	}
	return templates
}

func (o *DatabaseOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	for _, t := range o.templates() {
		if err := t.render(ctx, p, ident, o.Common.Destination); err != nil {
			return trace.Wrap(err, "rendering %s", t.name())
		}
	}

	return nil
}

func (o *DatabaseOutput) Init() error {
	subDirs, err := listSubdirectories(o.templates())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(o.Common.Destination.Init(subDirs))
}

func (o *DatabaseOutput) CheckAndSetDefaults() error {
	if o.Service == "" {
		return trace.BadParameter("service must not be empty")
	}

	if o.Subtype == UnspecifiedDatabaseSubtype {
		o.Subtype = StandardDatabaseSubtype
	}

	if !slices.Contains(databaseSubtypes, o.Subtype) {
		return trace.BadParameter("unrecognized subtype (%s)", o.Subtype)
	}

	return trace.Wrap(o.Common.CheckAndSetDefaults())
}

func (o *DatabaseOutput) GetDestination() bot.Destination {
	return o.Common.Destination
}

func (o *DatabaseOutput) GetRoles() []string {
	return o.Common.Roles
}

func (o *DatabaseOutput) Describe() []FileDescription {
	var fds []FileDescription
	for _, t := range o.templates() {
		fds = append(fds, t.describe()...)
	}

	return fds
}

func (o *DatabaseOutput) MarshalYAML() (interface{}, error) {
	type raw DatabaseOutput
	return marshalHeadered(raw(*o), DatabaseOutputType)
}

func (o *DatabaseOutput) String() string {
	return fmt.Sprintf("%s (%s)", DatabaseOutputType, o.Common.Destination)
}
