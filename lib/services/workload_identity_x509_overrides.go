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

package services

import (
	"context"
	"crypto/x509"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	apitypes "github.com/gravitational/teleport/api/types"
)

type WorkloadIdentityX509Overrides interface {
	GetX509IssuerOverride(ctx context.Context, name string) (*workloadidentityv1pb.X509IssuerOverride, error)
	ListX509IssuerOverrides(ctx context.Context, pageSize int, pageToken string) (_ []*workloadidentityv1pb.X509IssuerOverride, nextPageToken string, _ error)

	CreateX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error)
	UpdateX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error)
	UpsertX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error)
	DeleteX509IssuerOverride(ctx context.Context, name string) error
}

type rawSubjectPublicKeyInfo string

type workloadIdentityIssuerOverride struct {
	issuer   *x509.Certificate
	chainDER [][]byte
}

type ParsedWorkloadIdentityX509IssuerOverride map[rawSubjectPublicKeyInfo]workloadIdentityIssuerOverride

func (p ParsedWorkloadIdentityX509IssuerOverride) GetOverrideForIssuer(issuer *x509.Certificate) (*x509.Certificate, [][]byte) {
	o := p[rawSubjectPublicKeyInfo(issuer.RawSubjectPublicKeyInfo)]
	return o.issuer, o.chainDER
}

func ParseWorkloadIdentityX509IssuerOverride(resource *workloadidentityv1pb.X509IssuerOverride) (ParsedWorkloadIdentityX509IssuerOverride, error) {
	if expected, actual := apitypes.KindWorkloadIdentityX509IssuerOverride, resource.GetKind(); expected != actual {
		return nil, trace.BadParameter("expected kind %v, got %q", expected, actual)
	}
	if expected, actual := apitypes.V1, resource.GetVersion(); expected != actual {
		return nil, trace.BadParameter("expected version %v, got %q", expected, actual)
	}
	if expected, actual := "", resource.GetSubKind(); expected != actual {
		return nil, trace.BadParameter("expected sub_kind %v, got %q", expected, actual)
	}
	if name := resource.GetMetadata().GetName(); name == "" {
		return nil, trace.BadParameter("missing name")
	}
	if name := resource.GetMetadata().GetName(); name == "none" {
		return nil, trace.BadParameter("got reserved name \"none\"")
	}

	// TODO(espadolini): get rid of this limitation once the story around
	// multiple independent overrides and trust domains is more defined
	if name := resource.GetMetadata().GetName(); name != "default" {
		return nil, trace.BadParameter("expected name \"default\", got %q", name)
	}

	parsed := make(ParsedWorkloadIdentityX509IssuerOverride, len(resource.GetSpec().GetOverrides()))
	for _, override := range resource.GetSpec().GetOverrides() {
		issuer, err := x509.ParseCertificate(override.GetIssuer())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		spki := rawSubjectPublicKeyInfo(issuer.RawSubjectPublicKeyInfo)
		if _, alreadyExists := parsed[spki]; alreadyExists {
			return nil, trace.BadParameter("different overrides with the same public key are not allowed")
		}
		for _, certDER := range override.GetChain() {
			if _, err := x509.ParseCertificate(certDER); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		parsed[spki] = workloadIdentityIssuerOverride{
			issuer:   issuer,
			chainDER: override.GetChain(),
		}
	}

	return parsed, nil
}
