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
	"log/slog"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

const SPIFFESVIDOutputType = "spiffe-svid"

// Based on the default paths listed in
// https://github.com/spiffe/spiffe-helper/blob/main/README.md
const (
	SVIDPEMPath            = "svid.pem"
	SVIDKeyPEMPath         = "svid_key.pem"
	SVIDTrustBundlePEMPath = "svid_bundle.pem"
)

// SVIDRequestSANs is the configuration for the SANs of a single SVID request.
type SVIDRequestSANs struct {
	// DNS is the list of DNS names that are requested to be included in the SVID.
	DNS []string `yaml:"dns,omitempty"`
	// IP is the list of IP addresses that are requested to be included in the SVID.
	// These can be IPv4 or IPv6 addresses.
	IP []string `yaml:"ip,omitempty"`
}

// SVIDRequest is the configuration for a single SVID request.
type SVIDRequest struct {
	// Path is the SPIFFE ID path of the SVID. It should be prefixed with "/".
	Path string `yaml:"path,omitempty"`
	// Hint is the hint for the SVID that will be provided to consumers of the
	// SVID to help them identify it.
	Hint string `yaml:"hint,omitempty"`
	// SANS is the Subject Alternative Names that are requested to be included
	// in the SVID.
	SANS SVIDRequestSANs `yaml:"sans,omitempty"`
}

func (o SVIDRequest) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("path", o.Path),
		slog.String("hint", o.Hint),
		slog.Any("dns_sans", o.SANS.DNS),
		slog.Any("ip_sans", o.SANS.IP),
	)
}

// CheckAndSetDefaults checks the SVIDRequest values and sets any defaults.
func (o *SVIDRequest) CheckAndSetDefaults() error {
	switch {
	case o.Path == "":
		return trace.BadParameter("svid.path: should not be empty")
	case !strings.HasPrefix(o.Path, "/"):
		return trace.BadParameter("svid.path: should be prefixed with /")
	}
	for i, stringIP := range o.SANS.IP {
		ip := net.ParseIP(stringIP)
		if ip == nil {
			return trace.BadParameter(
				"ip_sans[%d]: invalid IP address %q", i, stringIP,
			)
		}
	}
	return nil
}

var (
	_ ServiceConfig = &SPIFFESVIDOutput{}
	_ Initable      = &SPIFFESVIDOutput{}
)

// JWTSVID the configuration for a single JWT SVID request as part of the SPIFFE
// SVID output.
type JWTSVID struct {
	// FileName is the name of the artifact/file the JWT should be written to.
	FileName string `yaml:"file_name"`
	// Audience is the audience of the JWT.
	Audience string `yaml:"audience"`
}

func (o JWTSVID) CheckAndSetDefaults() error {
	switch {
	case o.Audience == "":
		return trace.BadParameter("audience: should not be empty")
	case o.FileName == "":
		return trace.BadParameter("name: should not be empty")
	}
	return nil
}

// SPIFFESVIDOutput is the configuration for the SPIFFE SVID output.
// Emulates the output of https://github.com/spiffe/spiffe-helper
type SPIFFESVIDOutput struct {
	// Destination is where the credentials should be written to.
	Destination                  bot.Destination `yaml:"destination"`
	SVID                         SVIDRequest     `yaml:"svid"`
	IncludeFederatedTrustBundles bool            `yaml:"include_federated_trust_bundles,omitempty"`
	// JWTs is an optional list of audiences and file names to write JWT SVIDs
	// to.
	JWTs []JWTSVID `yaml:"jwts,omitempty"`

	// CertificateLifetime contains configuration for how long certificates will
	// last and the frequency at which they'll be renewed.
	CertificateLifetime CertificateLifetime `yaml:",inline"`
}

// Init initializes the destination.
func (o *SPIFFESVIDOutput) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// GetDestination returns the destination.
func (o *SPIFFESVIDOutput) GetDestination() bot.Destination {
	return o.Destination
}

// CheckAndSetDefaults checks the SPIFFESVIDOutput values and sets any defaults.
func (o *SPIFFESVIDOutput) CheckAndSetDefaults() error {
	if err := o.SVID.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating svid")
	}
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	for i, jwt := range o.JWTs {
		if err := jwt.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating jwts[%d]", i)
		}
	}
	return nil
}

// Describe returns the file descriptions for the SPIFFE SVID output.
func (o *SPIFFESVIDOutput) Describe() []FileDescription {
	fds := []FileDescription{
		{
			Name: SVIDPEMPath,
		},
		{
			Name: SVIDKeyPEMPath,
		},
		{
			Name: SVIDTrustBundlePEMPath,
		},
	}
	for _, jwt := range o.JWTs {
		fds = append(fds, FileDescription{Name: jwt.FileName})
	}
	return nil
}

func (o *SPIFFESVIDOutput) Type() string {
	return SPIFFESVIDOutputType
}

// MarshalYAML marshals the SPIFFESVIDOutput into YAML.
func (o *SPIFFESVIDOutput) MarshalYAML() (interface{}, error) {
	type raw SPIFFESVIDOutput
	return withTypeHeader((*raw)(o), SPIFFESVIDOutputType)
}

// UnmarshalYAML unmarshals the SPIFFESVIDOutput from YAML.
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

func (o *SPIFFESVIDOutput) GetCertificateLifetime() CertificateLifetime {
	return o.CertificateLifetime
}
