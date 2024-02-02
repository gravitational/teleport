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
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/services"
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

type SVIDRequestSANs struct {
	DNS []string `yaml:"dns,omitempty"`
	IP  []string `yaml:"ip,omitempty"`
}

type SVIDRequest struct {
	Path string          `yaml:"path,omitempty"`
	Hint string          `yaml:"hint,omitempty"`
	SANS SVIDRequestSANs `yaml:"sans,omitempty"`
}

// SPIFFESVIDOutput TODO
// Emulates the output of https://github.com/spiffe/spiffe-helper
type SPIFFESVIDOutput struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	SVID        SVIDRequest     `yaml:"svid"`
}

func (o *SPIFFESVIDOutput) Render(
	ctx context.Context, p provider, _ *identity.Identity,
) error {
	ctx, span := tracer.Start(
		ctx,
		"SPIFFESVIDOutput/Render",
	)
	defer span.End()

	spiffeCAs, err := p.GetCertAuthorities(ctx, types.SPIFFECA)
	if err != nil {
		return trace.Wrap(err)
	}

	privateKey, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return trace.Wrap(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return trace.Wrap(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return trace.Wrap(err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})

	res, err := p.SignX509SVIDs(ctx, &machineidv1pb.SignX509SVIDsRequest{
		Svids: []*machineidv1pb.SVIDRequest{
			{
				PublicKey:    pubPEM,
				SpiffeIdPath: o.SVID.Path,
				DnsSans:      o.SVID.SANS.DNS,
				IpSans:       o.SVID.SANS.IP,
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if len(res.Svids) != 1 {
		return trace.BadParameter("expected 1 svids returned, got %d", len(res.Svids))
	}
	svid := res.Svids[0]

	if err := o.Destination.Write(ctx, svidKeyPEMPath, privPEM); err != nil {
		return trace.Wrap(err, "writing svid key")
	}
	if err := o.Destination.Write(ctx, svidPEMPath, svid.Certificate); err != nil {
		return trace.Wrap(err, "writing svid certificate")
	}

	trustBundleBytes := &bytes.Buffer{}
	for _, ca := range spiffeCAs {
		for _, cert := range services.GetTLSCerts(ca) {
			// Values are already PEM encoded, so we just append to the buffer
			if _, err := trustBundleBytes.Write(cert); err != nil {
				return trace.Wrap(err, "writing trust bundle to buffer")
			}
		}
	}
	if err := o.Destination.Write(
		ctx, svidTrustBundlePEMPath, trustBundleBytes.Bytes(),
	); err != nil {
		return trace.Wrap(err, "writing svid trust bundle")
	}

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
	switch {
	case o.SVID.Path == "":
		return trace.BadParameter("svid.path: should not be empty")
	case !strings.HasPrefix(o.SVID.Path, "/"):
		return trace.BadParameter("svid.path: should be prefixed with /")
	}
	for i, stringIP := range o.SVID.SANS.IP {
		ip := net.ParseIP(stringIP)
		if ip == nil {
			return trace.BadParameter(
				"ipSans[%d]: invalid IP address %q", i, stringIP,
			)
		}
	}
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
