// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package subcav1_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	subcapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/subca/subcav1"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
)

// TestHACAOverrides tests CA override creation against an HA (High
// Availability), multi-HSM scenario.
func TestHACAOverrides(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const numServices = 4 // Arbitrary.
		const caType = types.DatabaseClientCA
		envs := subcav1.NewHAEnv(t, subcav1.HAEnvParams{
			Storage: subcaenv.EnvParams{
				CATypesToCreate: []types.CertAuthType{
					caType,
				},
			},
			NumServices: numServices,
		})

		// RPCs go against only the first env, watchers take care of the rest.
		env := envs[0]
		clock := env.Clock
		subCA := env.SubCAClient

		// Wait for watchers to durably block, then sleep past the First time.
		synctest.Wait()
		time.Sleep(subcav1.WatcherFirstDuration)
		synctest.Wait()

		caOverride := env.NewOverrideForCAType(t, caType)
		// Sanity check.
		require.Len(t, caOverride.GetSpec().GetCertificateOverrides(), numServices,
			"Unexpected number of caOverride.Spec.CertificateOverrides")

		// Attempt to enable without all CRLs fails
		{
			setDisabled(caOverride, false)
			_, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
				CaOverride: caOverride,
			}.Build())
			assert.ErrorContains(t, err, "cannot sign CRL for enabled override")

			setDisabled(caOverride, true)
		}

		// Disabled create is allowed, CRLs are created in background.
		createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), subcapb.CreateCertAuthorityOverrideRequest_builder{
			CaOverride: caOverride,
		}.Build())
		require.NoError(t, err)
		caOverride = createResp.GetCaOverride()
		assert.Len(t, caOverride.GetStatus().GetPublicKeyHashToCrl(), 1,
			"Unexpected number of caOverride.Status.PublicKeyHashToCrl")

		// Wait for CRL creation. Services often hit CompareFailed errors, so we'll
		// accelerate time until they coalesce.
		const maxAttempts = numServices * 2 // Arbitrary. Make sure it stops at some point.
		for range maxAttempts {
			synctest.Wait()
			time.Sleep(subcav1.ConditionalUpdateMaxStep)
			synctest.Wait()

			getResp, err := subCA.GetCertAuthorityOverride(t.Context(), subcapb.GetCertAuthorityOverrideRequest_builder{
				CaId: subcapb.CertAuthorityOverrideID_builder{
					CaType: caOverride.GetSubKind(),
				}.Build(),
			}.Build())
			require.NoError(t, err)
			caOverride = getResp.GetCaOverride()
			if len(caOverride.GetStatus().GetPublicKeyHashToCrl()) == numServices {
				break
			}
		}
		assert.Len(t, caOverride.GetStatus().GetPublicKeyHashToCrl(), numServices,
			"Unexpected number of caOverride.Status.PublicKeyHashToCrl, wait loop reached maxAttempts",
		)

		assertCRLs(t, caOverride, clock.Now())
	})
}

func setDisabled(caOverride *subcapb.CertAuthorityOverride, disabled bool) {
	for _, co := range caOverride.GetSpec().GetCertificateOverrides() {
		co.SetDisabled(disabled)
	}
}
