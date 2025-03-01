/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRotationZero verifies expected behavior of Rotataion.IsZero.
func TestRotationZero(t *testing.T) {
	tts := []struct {
		r *Rotation
		z bool
		d string
	}{
		{
			r: &Rotation{
				Phase: RotationPhaseStandby,
			},
			z: false,
			d: "non-empty rotation",
		},
		{
			r: &Rotation{},
			z: true,
			d: "empty but non-nil rotation",
		},
		{
			r: nil,
			z: true,
			d: "nil rotation",
		},
	}

	for _, tt := range tts {
		require.Equal(t, tt.z, tt.r.IsZero(), tt.d)
	}
}

// Test that the spec cluster name name will be set to match the resource name
func TestCheckAndSetDefaults(t *testing.T) {
	ca := CertAuthorityV2{
		Metadata: Metadata{Name: "caName"},
		Spec: CertAuthoritySpecV2{
			ClusterName: "clusterName",
			Type:        HostCA,
		},
	}
	err := ca.CheckAndSetDefaults()
	require.NoError(t, err)
	require.Equal(t, ca.Metadata.Name, ca.Spec.ClusterName)
}
