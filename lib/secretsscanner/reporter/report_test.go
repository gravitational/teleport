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

package reporter_test

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/defaults"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
	dtassert "github.com/gravitational/teleport/lib/devicetrust/assert"
	dtauthn "github.com/gravitational/teleport/lib/devicetrust/authn"
	dttestenv "github.com/gravitational/teleport/lib/devicetrust/testenv"
	secretsscannerclient "github.com/gravitational/teleport/lib/secretsscanner/client"
	"github.com/gravitational/teleport/lib/secretsscanner/reporter"
)

func TestReporter(t *testing.T) {
	// disable TLS routing check for tests
	t.Setenv(defaults.TLSRoutingConnUpgradeEnvVar, "false")
	deviceID := uuid.NewString()
	device, err := dttestenv.NewFakeMacOSDevice()
	require.NoError(t, err)

	tests := []struct {
		name              string
		preReconcileError error
		preAssertError    error
		assertErr         require.ErrorAssertionFunc
		report            []*accessgraphsecretsv1pb.PrivateKey
		want              []*accessgraphsecretsv1pb.PrivateKey
	}{
		{
			name:      "success",
			report:    newPrivateKeys(t, deviceID),
			want:      newPrivateKeys(t, deviceID),
			assertErr: require.NoError,
		},
		{
			name:           "pre-assert error",
			preAssertError: errors.New("pre-assert error"),
			report:         newPrivateKeys(t, deviceID),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "pre-assert error")
			},
		},
		{
			name:              "pre-reconcile error",
			preReconcileError: errors.New("pre-reconcile error"),
			report:            newPrivateKeys(t, deviceID),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "pre-reconcile error")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := setup(
				t,
				withDevice(deviceID, device),
				withPreReconcileError(tt.preReconcileError),
				withPreAssertError(tt.preAssertError),
			)

			ctx := context.Background()

			client, err := secretsscannerclient.NewSecretsScannerServiceClient(ctx,
				secretsscannerclient.ClientConfig{
					ProxyServer: e.secretsScannerAddr,
					Insecure:    true,
				},
			)
			require.NoError(t, err)

			r, err := reporter.New(
				reporter.Config{
					Log:       slog.Default(),
					Client:    client,
					BatchSize: 1, /* batch size for tests */
					AssertCeremonyBuilder: func() (*dtassert.Ceremony, error) {
						return dtassert.NewCeremony(
							dtassert.WithNewAuthnCeremonyFunc(
								func() *dtauthn.Ceremony {
									return &dtauthn.Ceremony{
										GetDeviceCredential: func() (*devicepb.DeviceCredential, error) {
											return device.GetDeviceCredential(), nil
										},
										CollectDeviceData:            device.CollectDeviceData,
										SignChallenge:                device.SignChallenge,
										SolveTPMAuthnDeviceChallenge: device.SolveTPMAuthnDeviceChallenge,
										GetDeviceOSType:              device.GetDeviceOSType,
									}
								},
							),
						)
					},
				},
			)
			require.NoError(t, err)

			err = r.ReportPrivateKeys(ctx, tt.report)
			tt.assertErr(t, err)

			got := e.service.privateKeysReported
			sortPrivateKeys(got)
			sortPrivateKeys(tt.want)

			diff := cmp.Diff(tt.want, got, protocmp.Transform())
			require.Empty(t, diff, "ReportPrivateKeys keys mismatch (-got +want)")

		})
	}
}

func sortPrivateKeys(keys []*accessgraphsecretsv1pb.PrivateKey) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Metadata.Name < keys[j].Metadata.Name
	})
}

func newPrivateKeys(t *testing.T, deviceID string) []*accessgraphsecretsv1pb.PrivateKey {
	t.Helper()
	var pks []*accessgraphsecretsv1pb.PrivateKey
	for i := 0; i < 10; i++ {
		pk, err := accessgraph.NewPrivateKey(
			&accessgraphsecretsv1pb.PrivateKeySpec{
				PublicKeyFingerprint: "key" + strconv.Itoa(i),
				DeviceId:             deviceID,
				PublicKeyMode:        accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED,
			},
		)
		require.NoError(t, err)
		pks = append(pks, pk)
	}

	return pks
}
