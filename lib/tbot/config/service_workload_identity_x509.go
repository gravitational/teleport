// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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

const WorkloadIdentityX509OutputType = "workload-identity-x509"

var (
	_ ServiceConfig = &WorkloadIdentityX509Service{}
	_ Initable      = &WorkloadIdentityX509Service{}
)

// Based on the default paths listed in
// https://github.com/spiffe/spiffe-helper/blob/main/README.md
const (
	SVIDPEMPath            = "svid.pem"
	SVIDKeyPEMPath         = "svid_key.pem"
	SVIDTrustBundlePEMPath = "svid_bundle.pem"
	SVIDCRLPemPath         = "svid_crl.pem"
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

// WorkloadIdentitySelector allows the user to select which WorkloadIdentity
// resource should be used.
//
// Only one of Name or Labels can be set.
type WorkloadIdentitySelector struct {
	// Name is the name of a specific WorkloadIdentity resource.
	Name string `yaml:"name"`
	// Labels is a set of labels that the WorkloadIdentity resource must have.
	Labels map[string][]string `yaml:"labels,omitempty"`
}

// CheckAndSetDefaults checks the WorkloadIdentitySelector values and sets any
// defaults.
func (s *WorkloadIdentitySelector) CheckAndSetDefaults() error {
	switch {
	case s.Name == "" && len(s.Labels) == 0:
		return trace.BadParameter("one of ['name', 'labels'] must be set")
	case s.Name != "" && len(s.Labels) > 0:
		return trace.BadParameter("at most one of ['name', 'labels'] can be set")
	}
	for k, v := range s.Labels {
		if len(v) == 0 {
			return trace.BadParameter("labels[%s]: must have at least one value", k)
		}
	}
	return nil
}

// WorkloadIdentityX509Service is the configuration for the WorkloadIdentityX509Service
// Emulates the output of https://github.com/spiffe/spiffe-helper
type WorkloadIdentityX509Service struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Selector is the selector for the WorkloadIdentity resource that will be
	// used to issue WICs.
	Selector WorkloadIdentitySelector `yaml:"selector"`
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// IncludeFederatedTrustBundles controls whether to include federated trust
	// bundles in the output.
	IncludeFederatedTrustBundles bool `yaml:"include_federated_trust_bundles,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime CredentialLifetime `yaml:",inline"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o WorkloadIdentityX509Service) GetName() string {
	return o.Name
}

// Init initializes the destination.
func (o *WorkloadIdentityX509Service) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

// GetDestination returns the destination.
func (o *WorkloadIdentityX509Service) GetDestination() bot.Destination {
	return o.Destination
}

// CheckAndSetDefaults checks the SPIFFESVIDOutput values and sets any defaults.
func (o *WorkloadIdentityX509Service) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if err := o.Selector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating selector")
	}
	return nil
}

// Describe returns the file descriptions for the WorkloadIdentityX509Service.
func (o *WorkloadIdentityX509Service) Describe() []FileDescription {
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
		{
			Name: SVIDCRLPemPath,
		},
	}
	return fds
}

func (o *WorkloadIdentityX509Service) Type() string {
	return WorkloadIdentityX509OutputType
}

// MarshalYAML marshals the WorkloadIdentityX509Service into YAML.
func (o *WorkloadIdentityX509Service) MarshalYAML() (any, error) {
	type raw WorkloadIdentityX509Service
	return withTypeHeader((*raw)(o), WorkloadIdentityX509OutputType)
}

// UnmarshalYAML unmarshals the WorkloadIdentityX509Service from YAML.
func (o *WorkloadIdentityX509Service) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw WorkloadIdentityX509Service
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *WorkloadIdentityX509Service) GetCredentialLifetime() CredentialLifetime {
	lt := o.CredentialLifetime
	lt.skipMaxTTLValidation = true
	return lt
}
