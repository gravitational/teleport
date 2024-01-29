/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const SPIFFESVIDOutputType = "spiffe-svid"

// Based on the default paths listed in
// https://github.com/spiffe/spiffe-helper/blob/main/README.md
const (
	svidPEMPath            = "svid.pem"
	svidKeyPEMPath         = "svid_key.pem"
	svidTrustBundlePEMPath = "svid_bundle.pem"
)

// SPIFFESVIDOutput TODO
// Emulates the output of https://github.com/spiffe/spiffe-helper
type SPIFFESVIDOutput struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
}

func (o *SPIFFESVIDOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	ctx, span := tracer.Start(
		ctx,
		"SPIFFESVIDOutput/Render",
	)
	defer span.End()

	// dest := o.GetDestination()
	// TODO: Write cert, key and trust bundle to destination

	return nil
}

func (o *SPIFFESVIDOutput) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *SPIFFESVIDOutput) GetDestination() bot.Destination {
	return o.Destination
}

func (o *SPIFFESVIDOutput) GetRoles() []string {
	// Always use all roles default - which is empty
	return []string{}
}

func (o *SPIFFESVIDOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (o *SPIFFESVIDOutput) Describe() []FileDescription {
	return []FileDescription{
		{
			Name: svidPEMPath,
		},
		{
			Name: svidKeyPEMPath,
		},
		{
			Name: svidTrustBundlePEMPath,
		},
	}
}

func (o *SPIFFESVIDOutput) MarshalYAML() (interface{}, error) {
	type raw SPIFFESVIDOutput
	return withTypeHeader((*raw)(o), SPIFFESVIDOutputType)
}

func (o *SPIFFESVIDOutput) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw SPIFFESVIDOutput
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *SPIFFESVIDOutput) String() string {
	return fmt.Sprintf("%s (%s)", SPIFFESVIDOutputType, o.GetDestination())
}
