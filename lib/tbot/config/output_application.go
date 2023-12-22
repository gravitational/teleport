/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package config

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const ApplicationOutputType = "application"

type ApplicationOutput struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	AppName string `yaml:"app_name"`

	// SpecificTLSExtensions creates additional outputs named `tls.crt`,
	// `tls.key` and `tls.cas`. This is unneeded for most clients which can
	// be configured with specific paths to use, but exists for compatibility.
	SpecificTLSExtensions bool `yaml:"specific_tls_naming"`
}

func (o *ApplicationOutput) templates() []template {
	templates := []template{
		&templateTLSCAs{},
		&templateIdentity{},
	}
	if o.SpecificTLSExtensions {
		templates = append(templates, &templateTLS{
			caCertType: types.HostCA,
		})
	}
	return templates
}

func (o *ApplicationOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	ctx, span := tracer.Start(
		ctx,
		"ApplicationOutput/Render",
	)
	defer span.End()

	if err := identity.SaveIdentity(ctx, ident, o.Destination, identity.DestinationKinds()...); err != nil {
		return trace.Wrap(err, "persisting identity")
	}

	for _, t := range o.templates() {
		if err := t.render(ctx, p, ident, o.Destination); err != nil {
			return trace.Wrap(err, "rendering template %s", t.name())
		}
	}

	return nil
}

func (o *ApplicationOutput) Init(ctx context.Context) error {
	subDirs, err := listSubdirectories(o.templates())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(o.Destination.Init(ctx, subDirs))
}

func (o *ApplicationOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if o.AppName == "" {
		return trace.BadParameter("app_name must not be empty")
	}

	return nil
}

func (o *ApplicationOutput) GetDestination() bot.Destination {
	return o.Destination
}

func (o *ApplicationOutput) GetRoles() []string {
	return o.Roles
}

func (o *ApplicationOutput) Describe() []FileDescription {
	var fds []FileDescription
	for _, t := range o.templates() {
		fds = append(fds, t.describe()...)
	}

	return fds
}

func (o *ApplicationOutput) MarshalYAML() (interface{}, error) {
	type raw ApplicationOutput
	return withTypeHeader((*raw)(o), ApplicationOutputType)
}

func (o *ApplicationOutput) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw ApplicationOutput
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *ApplicationOutput) String() string {
	return fmt.Sprintf("%s (%s)", ApplicationOutputType, o.GetDestination())
}
