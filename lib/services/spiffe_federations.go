// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package services

import (
	"context"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

// SPIFFEFederations is an interface over the SPIFFEFederations service. This
// interface may also be implemented by a client to allow remote and local
// consumers to access the resource in a similar way.
type SPIFFEFederations interface {
	// GetSPIFFEFederation gets a SPIFFE Federation by name.
	GetSPIFFEFederation(
		ctx context.Context, name string,
	) (*machineidv1.SPIFFEFederation, error)
	// ListSPIFFEFederations lists all SPIFFE Federations using Google style
	// pagination.
	ListSPIFFEFederations(
		ctx context.Context, pageSize int, lastToken string,
	) ([]*machineidv1.SPIFFEFederation, string, error)
	// CreateSPIFFEFederation creates a new SPIFFE Federation.
	CreateSPIFFEFederation(
		ctx context.Context, spiffeFederation *machineidv1.SPIFFEFederation,
	) (*machineidv1.SPIFFEFederation, error)
	// DeleteSPIFFEFederation deletes a SPIFFE Federation by name.
	DeleteSPIFFEFederation(ctx context.Context, name string) error
	// UpdateSPIFFEFederation updates a SPIFFE Federation. It will not act if the resource is not found
	// or where the revision does not match.
	UpdateSPIFFEFederation(
		ctx context.Context, spiffeFederation *machineidv1.SPIFFEFederation,
	) (*machineidv1.SPIFFEFederation, error)
}

// MarshalSPIFFEFederation marshals the SPIFFEFederation object into a JSON byte
// array.
func MarshalSPIFFEFederation(object *machineidv1.SPIFFEFederation, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalSPIFFEFederation unmarshals the SPIFFEFederation object from a
// JSON byte array.
func UnmarshalSPIFFEFederation(
	data []byte, opts ...MarshalOption,
) (*machineidv1.SPIFFEFederation, error) {
	return UnmarshalProtoResource[*machineidv1.SPIFFEFederation](data, opts...)
}

// ValidateSPIFFEFederation validates the SPIFFEFederation object.
func ValidateSPIFFEFederation(s *machineidv1.SPIFFEFederation) error {
	switch {
	case s == nil:
		return trace.BadParameter("object cannot be nil")
	case s.Version != types.V1:
		return trace.BadParameter("version: only %q is supported", types.V1)
	case s.Kind != types.KindSPIFFEFederation:
		return trace.BadParameter("kind: must be %q", types.KindSPIFFEFederation)
	case s.Metadata == nil:
		return trace.BadParameter("metadata: is required")
	case s.Metadata.Name == "":
		return trace.BadParameter("metadata.name: is required")
	case s.Spec == nil:
		return trace.BadParameter("spec: is required")
	case s.Spec.BundleSource == nil:
		return trace.BadParameter("spec.bundle_source: is required")
	case s.Spec.BundleSource.HttpsWeb != nil && s.Spec.BundleSource.Static != nil:
		return trace.BadParameter("spec.bundle_source: at most one of https_web or static can be set")
	case s.Spec.BundleSource.HttpsWeb == nil && s.Spec.BundleSource.Static == nil:
		return trace.BadParameter("spec.bundle_source: at least one of https_web or static must be set")
	}

	// Validate name is valid SPIFFE Trust Domain name without the "spiffe://"
	name := s.Metadata.Name
	if strings.HasPrefix(name, "spiffe://") {
		return trace.BadParameter(
			"metadata.name: must not include the spiffe:// prefix",
		)
	}
	td, err := spiffeid.TrustDomainFromString(name)
	if err != nil {
		return trace.Wrap(err, "validating metadata.name")
	}

	// Validate Static
	if s.Spec.BundleSource.Static != nil {
		if s.Spec.BundleSource.Static.Bundle == "" {
			return trace.BadParameter("spec.bundle_source.static.bundle: is required")
		}
		// Validate contents
		// TODO(noah): Is this a bit intense to run on every validation?
		// This could easily be moved into reconciliation...
		_, err := spiffebundle.Parse(td, []byte(s.Spec.BundleSource.Static.Bundle))
		if err != nil {
			return trace.Wrap(err, "validating spec.bundle_source.static.bundle")
		}
	}

	// Validate HTTPSWeb
	if s.Spec.BundleSource.HttpsWeb != nil {
		if s.Spec.BundleSource.HttpsWeb.BundleEndpointUrl == "" {
			return trace.BadParameter("spec.bundle_source.https_web.bundle_endpoint_url: is required")
		}
		_, err := url.Parse(s.Spec.BundleSource.HttpsWeb.BundleEndpointUrl)
		if err != nil {
			return trace.Wrap(err, "validating spec.bundle_source.https_web.bundle_endpoint_url")
		}
	}

	// Ensure that all key status fields are set if any are set. This is a safeguard against weird inconsistent states
	// where some fields are set and others are not.
	currentBundleSet := s.Status.GetCurrentBundle() != ""
	currentBundledSyncedAtSet := s.Status.GetCurrentBundleSyncedAt() != nil
	currentBundleSyncedFromSet := s.Status.GetCurrentBundleSyncedFrom() != nil
	anyStatusFieldSet := currentBundleSet || currentBundledSyncedAtSet || currentBundleSyncedFromSet
	allStatusFieldsSet := currentBundleSet && currentBundledSyncedAtSet && currentBundleSyncedFromSet
	if anyStatusFieldSet && !allStatusFieldsSet {
		return trace.BadParameter("status: all of ['current_bundle', 'current_bundle_synced_at', 'current_bundle_synced_from'] must be set if any are set")
	}

	return nil
}
