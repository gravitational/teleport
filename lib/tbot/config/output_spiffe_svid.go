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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"gopkg.in/yaml.v3"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1/experiment"
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

// SPIFFESVIDOutput is the configuration for the SPIFFE SVID output.
// Emulates the output of https://github.com/spiffe/spiffe-helper
type SPIFFESVIDOutput struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	SVID        SVIDRequest     `yaml:"svid"`
}

type spiffeSVIDSigner interface {
	SignX509SVIDs(ctx context.Context, req *machineidv1pb.SignX509SVIDsRequest, opts ...grpc.CallOption) (*machineidv1pb.SignX509SVIDsResponse, error)
}

func GenerateSVID(
	ctx context.Context,
	signer spiffeSVIDSigner,
	reqs []SVIDRequest,
	ttl time.Duration,
) (*machineidv1pb.SignX509SVIDsResponse, *rsa.PrivateKey, error) {
	privateKey, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	svids := make([]*machineidv1pb.SVIDRequest, 0, len(reqs))
	for _, req := range reqs {
		svids = append(svids, &machineidv1pb.SVIDRequest{
			PublicKey:    pubBytes,
			SpiffeIdPath: req.Path,
			DnsSans:      req.SANS.DNS,
			IpSans:       req.SANS.IP,
			Hint:         req.Hint,
			Ttl:          durationpb.New(ttl),
		})
	}

	res, err := signer.SignX509SVIDs(ctx, &machineidv1pb.SignX509SVIDsRequest{
		Svids: svids,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return res, privateKey, nil
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

	res, privateKey, err := GenerateSVID(
		ctx,
		p,
		[]SVIDRequest{o.SVID},
		// For TTL, we use the one globally configured.
		p.Config().CertificateTTL,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return trace.Wrap(err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})

	svid := res.Svids[0]
	if err := o.Destination.Write(ctx, svidKeyPEMPath, privPEM); err != nil {
		return trace.Wrap(err, "writing svid key")
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: svid.Certificate,
	})
	if err := o.Destination.Write(ctx, svidPEMPath, certPEM); err != nil {
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
	if !experiment.Enabled() {
		return trace.AccessDenied("workload identity has not been enabled")
	}

	if err := o.SVID.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating svid")
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
	return fmt.Sprintf("%s (%s) (%s)", SPIFFESVIDOutputType, o.SVID.Path, o.GetDestination())
}
