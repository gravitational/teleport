/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidateBeam(t *testing.T) {
	t.Parallel()

	require.NoError(t, services.ValidateBeam(testBeam("beam-alias")))

	unrestrictedBeam := testBeam("beam-alias")
	unrestrictedBeam.Spec.Egress = beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED
	unrestrictedBeam.Spec.AllowedDomains = nil
	require.NoError(t, services.ValidateBeam(unrestrictedBeam))

	testCases := map[string]struct {
		beam  *beamsv1.Beam
		modFn func(*beamsv1.Beam)
		err   string
	}{
		"nil beam": {
			beam: nil,
			err:  "beam must not be nil",
		},
		"wrong version": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Version = ""
			},
			err: `version: only supports version "v1", got ""`,
		},
		"wrong kind": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Kind = ""
			},
			err: `kind: must be "beam", got ""`,
		},
		"missing metadata": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Metadata = nil
			},
			err: "metadata: is required",
		},
		"missing metadata name": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Metadata.Name = ""
			},
			err: "metadata.name: is required",
		},
		"missing spec": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec = nil
			},
			err: "spec: is required",
		},
		"unspecified egress": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec.Egress = beamsv1.EgressMode_EGRESS_MODE_UNSPECIFIED
			},
			err: "spec.egress: must be EGRESS_MODE_RESTRICTED or EGRESS_MODE_UNRESTRICTED",
		},
		"invalid egress": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec.Egress = beamsv1.EgressMode(42)
			},
			err: "spec.egress: must be EGRESS_MODE_RESTRICTED or EGRESS_MODE_UNRESTRICTED",
		},
		"allowed domains with unrestricted egress": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec.Egress = beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED
			},
			err: "spec.allowed_domains: may only be set when spec.egress is EGRESS_MODE_RESTRICTED",
		},
		"empty allowed domain": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec.AllowedDomains = []string{""}
			},
			err: `spec.allowed_domains[0]: "" must be a fully qualified domain name ending with '.'`,
		},
		"invalid allowed domain": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec.AllowedDomains = []string{"Example.COM."}
			},
			err: `spec.allowed_domains[0]: "Example.COM." is invalid`,
		},
		"wildcard allowed domain": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec.AllowedDomains = []string{"*.example.com."}
			},
			err: `spec.allowed_domains[0]: "*.example.com." is invalid`,
		},
		"allowed domain missing trailing dot": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec.AllowedDomains = []string{"example.com"}
			},
			err: `spec.allowed_domains[0]: "example.com" must be a fully qualified domain name ending with '.'`,
		},
		"allowed domain is not fqdn": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Spec.AllowedDomains = []string{"localhost."}
			},
			err: `spec.allowed_domains[0]: "localhost." must be a fully qualified domain name ending with '.'`,
		},
		"missing status": {
			beam: testBeam("beam-alias"),
			modFn: func(b *beamsv1.Beam) {
				b.Status = nil
			},
			err: "status: is required",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			beam := tc.beam
			if tc.modFn != nil {
				tc.modFn(beam)
			}

			err := services.ValidateBeam(beam)
			require.ErrorContains(t, err, tc.err)
			require.True(t, trace.IsBadParameter(err))
		})
	}
}

func testBeam(alias string) *beamsv1.Beam {
	return &beamsv1.Beam{
		Kind:    types.KindBeam,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    uuid.NewString(),
			Expires: timestamppb.New(time.Now().Add(time.Hour)),
		},
		Spec: &beamsv1.BeamSpec{
			Egress:         beamsv1.EgressMode_EGRESS_MODE_RESTRICTED,
			AllowedDomains: []string{"example.com."},
		},
		Status: &beamsv1.BeamStatus{
			User:  "alice",
			Alias: alias,
		},
	}
}
