package accessgraph

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accessgraph"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/devicetrust/assert"
	dtauthn "github.com/gravitational/teleport/lib/devicetrust/authn"
	dttestenv "github.com/gravitational/teleport/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestReportAuthorizedKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		authorizer authz.Authorizer
		failEarly  bool
		// storedAuthorizedKeys is the list of authorized keys that are stored in the database before the test.
		storedAuthorizedKeys []*accessgraphsecretsv1pb.AuthorizedKey
		// newAuthorizedKeys is the list of authorized keys that are reported by the client.
		newAuthorizedKeys []*accessgraphsecretsv1pb.AuthorizedKey
		// expectedAuthorizedKeys is the list of authorized keys that are expected to be stored in the database after the test.
		expectedAuthorizedKeys []*accessgraphsecretsv1pb.AuthorizedKey
		assertErr              func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient)
		assertUsageReports     func(t *testing.T, f *fakeUsageReporter)
	}{
		{
			desc: "failure because identity isn't node",
			authorizer: fakeAuthorizer{
				identity: genBuiltinRoleAuthzContext("test-machine", clusterName, types.RoleProxy),
			},
			failEarly: true,
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient) {
				_, err := stream.Recv()
				require.ErrorContains(t, err, " only nodes are allowed to report authorized keys")
			},
		},
		{
			desc: "failure because identity isn't local node",
			authorizer: fakeAuthorizer{
				identity: genRemoteRoleAuthzContext("test-machine", clusterName, types.RoleProxy),
			},
			failEarly: true,
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient) {
				_, err := stream.Recv()
				require.ErrorContains(t, err, " only nodes are allowed to report authorized keys")
			},
		},
		{
			desc: "failure because identity isn't node",
			authorizer: fakeAuthorizer{
				identity: genBuiltinRoleAuthzContext("test-machine", clusterName, types.RoleProxy),
			},
			failEarly: true,
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient) {
				_, err := stream.Recv()
				require.ErrorContains(t, err, " only nodes are allowed to report authorized keys")
			},
		},

		{
			desc: "client sends hostID that doesn't match the identity",
			authorizer: fakeAuthorizer{
				identity: genBuiltinRoleAuthzContext("host1", clusterName, types.RoleNode),
			},
			failEarly: true,
			newAuthorizedKeys: []*accessgraphsecretsv1pb.AuthorizedKey{
				newAuthorizedKey(t, "host2", "fingerprint1"),
			},
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient) {
				_, err := stream.Recv()
				require.ErrorContains(t, err, "host ID mismatch")
			},
		},

		{
			desc: "successful with no prior keys",
			authorizer: fakeAuthorizer{
				identity: genBuiltinRoleAuthzContext("host1", clusterName, types.RoleNode),
			},
			newAuthorizedKeys: []*accessgraphsecretsv1pb.AuthorizedKey{
				newAuthorizedKey(t, "host1", "fingerprint1"),
				newAuthorizedKey(t, "host1", "fingerprint2"),
			},
			expectedAuthorizedKeys: []*accessgraphsecretsv1pb.AuthorizedKey{
				newAuthorizedKey(t, "host1", "fingerprint1"),
				newAuthorizedKey(t, "host1", "fingerprint2"),
			},
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient) {
				_, err := stream.Recv()
				require.ErrorIs(t, err, io.EOF)
			},
			assertUsageReports: func(t *testing.T, f *fakeUsageReporter) {
				require.Len(t, f.events, 1)
				evt := f.events[0].Anonymize(&fakeAnonymizer{})
				require.NotNil(t, evt.GetAccessGraphSecretsScanAuthorizedKeys())
				authEvt := evt.GetAccessGraphSecretsScanAuthorizedKeys()
				require.Equal(t, "HOST1", authEvt.HostId) // fakeAnonymizer should uppercase the host id
				require.EqualValues(t, 2, authEvt.TotalKeys)
			},
		},
		{
			desc: "successful with prior keys",
			authorizer: fakeAuthorizer{
				identity: genBuiltinRoleAuthzContext("host1", clusterName, types.RoleNode),
			},
			storedAuthorizedKeys: []*accessgraphsecretsv1pb.AuthorizedKey{
				newAuthorizedKey(t, "host1", "fingerprint5"),
			},
			newAuthorizedKeys: []*accessgraphsecretsv1pb.AuthorizedKey{
				newAuthorizedKey(t, "host1", "fingerprint1"),
				newAuthorizedKey(t, "host1", "fingerprint2"),
			},
			expectedAuthorizedKeys: []*accessgraphsecretsv1pb.AuthorizedKey{
				newAuthorizedKey(t, "host1", "fingerprint1"),
				newAuthorizedKey(t, "host1", "fingerprint2"),
			},
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient) {
				_, err := stream.Recv()
				require.ErrorIs(t, err, io.EOF)
			},
			assertUsageReports: func(t *testing.T, f *fakeUsageReporter) {
				require.Len(t, f.events, 1)
				evt := f.events[0].Anonymize(&fakeAnonymizer{})
				require.NotNil(t, evt.GetAccessGraphSecretsScanAuthorizedKeys())
				authEvt := evt.GetAccessGraphSecretsScanAuthorizedKeys()
				require.Equal(t, "HOST1", authEvt.HostId) // fakeAnonymizer should uppercase the host id
				require.EqualValues(t, 2, authEvt.TotalKeys)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			env := setup(
				t,
				withAuthorizer(tt.authorizer),
				withAuthorizedKeys(tt.storedAuthorizedKeys),
			)

			stream, err := env.secretsScannerClient.ReportAuthorizedKeys(ctx)
			require.NoError(t, err)

			err = stream.Send(accessgraphsecretsv1pb.ReportAuthorizedKeysRequest_builder{
				Keys:      tt.newAuthorizedKeys,
				Operation: accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_ADD,
			}.Build())
			/* ignore error if it's EOF, as it's expected */
			if !errors.Is(err, io.EOF) {
				require.NoError(t, err)
			}

			if tt.failEarly {
				tt.assertErr(t, stream)
				return
			}

			err = stream.Send(accessgraphsecretsv1pb.ReportAuthorizedKeysRequest_builder{
				Operation: accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_SYNC,
			}.Build())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				got := collectAll(t, ctx, env.accessGraphSecretsStorage.ListAllAuthorizedKeys)
				return len(got) == len(tt.newAuthorizedKeys)
			}, 5*time.Second, 100*time.Millisecond)

			err = stream.CloseSend()
			/* ignore error if it's EOF, as it's expected */
			if !errors.Is(err, io.EOF) {
				require.NoError(t, err)
			}

			tt.assertErr(t, stream)

			got := collectAll(t, ctx, env.accessGraphSecretsStorage.ListAllAuthorizedKeys)

			require.Empty(t,
				cmp.Diff(tt.expectedAuthorizedKeys, got,
					cmpopts.SortSlices(func(a, b *accessgraphsecretsv1pb.AuthorizedKey) bool {
						return a.GetMetadata().GetName() < b.GetMetadata().GetName()
					}),
					protocmp.Transform(),
					protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"),
				),
			)

			if tt.assertUsageReports != nil {
				tt.assertUsageReports(t, env.usageReporter)
			}

		})
	}
}

func TestReportPrivateKeys(t *testing.T) {
	var (
		deviceID = uuid.NewString()
	)
	setDeviceIdPrivateKey := func(key *accessgraphsecretsv1pb.PrivateKey) *accessgraphsecretsv1pb.PrivateKey {
		key.GetSpec().SetDeviceId(deviceID)
		return key
	}

	linux := dttestenv.NewFakeLinuxDevice()
	mac, err := dttestenv.NewFakeMacOSDevice()
	require.NoError(t, err)

	tests := []struct {
		desc             string
		failEarly        bool
		useDiffDevice    bool
		registeredDevice dttestenv.FakeDevice
		// storedPrivateKeys is the list of private keys that are stored in the database before the test.
		storedPrivateKeys []*accessgraphsecretsv1pb.PrivateKey
		// newPrivateKeys is the list of private keys that are reported by the client.
		newPrivateKeys []*accessgraphsecretsv1pb.PrivateKey
		// expectedPrivateKeys is the list of private keys that are expected to be stored in the database after the test.
		expectedPrivateKeys []*accessgraphsecretsv1pb.PrivateKey
		assertErr           func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient)
		assertUsageReports  func(t *testing.T, f *fakeUsageReporter)
	}{
		{
			desc:             "macOS successful with no prior keys",
			registeredDevice: mac,
			newPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				newPrivateKey(t, "fingerprint1"),
				newPrivateKey(t, "fingerprint2"),
			},
			expectedPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint1")),
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint2")),
			},
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) {
				_, err := stream.Recv()
				require.ErrorIs(t, err, io.EOF)
			},
			assertUsageReports: func(t *testing.T, f *fakeUsageReporter) {
				require.Len(t, f.events, 1)
				evt := f.events[0].Anonymize(&fakeAnonymizer{})
				require.NotNil(t, evt.GetAccessGraphSecretsScanSshPrivateKeys())
				authEvt := evt.GetAccessGraphSecretsScanSshPrivateKeys()
				require.Equal(t, strings.ToUpper(deviceID), authEvt.DeviceId) // fakeAnonymizer should uppercase the host id
				require.Equal(t, mac.GetDeviceOSType().String(), authEvt.DeviceOsType)
				require.EqualValues(t, 2, authEvt.TotalKeys)
			},
		},
		{
			desc:             "linux successful with no prior keys",
			registeredDevice: linux,
			newPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				newPrivateKey(t, "fingerprint1"),
				newPrivateKey(t, "fingerprint2"),
			},
			expectedPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint1")),
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint2")),
			},
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) {
				_, err := stream.Recv()
				require.ErrorIs(t, err, io.EOF)
			},
			assertUsageReports: func(t *testing.T, f *fakeUsageReporter) {
				require.Len(t, f.events, 1)
				evt := f.events[0].Anonymize(&fakeAnonymizer{})
				require.NotNil(t, evt.GetAccessGraphSecretsScanSshPrivateKeys())
				authEvt := evt.GetAccessGraphSecretsScanSshPrivateKeys()
				require.Equal(t, strings.ToUpper(deviceID), authEvt.DeviceId) // fakeAnonymizer should uppercase the host id
				require.Equal(t, linux.GetDeviceOSType().String(), authEvt.DeviceOsType)
				require.EqualValues(t, 2, authEvt.TotalKeys)
			},
		},
		{
			desc:             "macOS successful with prior keys",
			registeredDevice: mac,
			storedPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint5")),
			},
			newPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				newPrivateKey(t, "fingerprint1"),
				newPrivateKey(t, "fingerprint2"),
			},
			expectedPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint1")),
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint2")),
			},
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) {
				_, err := stream.Recv()
				require.ErrorIs(t, err, io.EOF)
			},
			assertUsageReports: func(t *testing.T, f *fakeUsageReporter) {
				require.Len(t, f.events, 1)
				evt := f.events[0].Anonymize(&fakeAnonymizer{})
				require.NotNil(t, evt.GetAccessGraphSecretsScanSshPrivateKeys())
				authEvt := evt.GetAccessGraphSecretsScanSshPrivateKeys()
				require.Equal(t, strings.ToUpper(deviceID), authEvt.DeviceId) // fakeAnonymizer should uppercase the host id
				require.Equal(t, mac.GetDeviceOSType().String(), authEvt.DeviceOsType)
				require.EqualValues(t, 2, authEvt.TotalKeys)
			},
		},
		{
			desc:             "linux successful with prior keys",
			registeredDevice: linux,
			storedPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint5")),
			},
			newPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				newPrivateKey(t, "fingerprint1"),
				newPrivateKey(t, "fingerprint2"),
			},
			expectedPrivateKeys: []*accessgraphsecretsv1pb.PrivateKey{
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint1")),
				setDeviceIdPrivateKey(newPrivateKey(t, "fingerprint2")),
			},
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) {
				_, err := stream.Recv()
				require.ErrorIs(t, err, io.EOF)
			},
		},
		{
			desc:             "failure with different device",
			registeredDevice: mac,
			failEarly:        true,
			useDiffDevice:    true,
			assertErr: func(t *testing.T, stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) {
				_, err := stream.Recv()
				require.ErrorContains(t, err, "device not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			env := setup(
				t,
				withPrivateKeys(tt.storedPrivateKeys),
				withDevice(deviceID, tt.registeredDevice),
				withModules(&modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.DeviceTrust:            {Enabled: true},
							entitlements.MobileDeviceManagement: {Enabled: true},
						},
					},
				}),
			)
			dev := tt.registeredDevice
			var err error
			if tt.useDiffDevice {
				dev, err = dttestenv.NewFakeMacOSDevice()
				require.NoError(t, err)
			}
			ceremony, err := assert.NewCeremony(assert.WithNewAuthnCeremonyFunc(
				func() *dtauthn.Ceremony {
					return &dtauthn.Ceremony{
						GetDeviceCredential: func() (*devicepb.DeviceCredential, error) {
							return dev.GetDeviceCredential(), nil
						},
						CollectDeviceData:            dev.CollectDeviceData,
						SignChallenge:                dev.SignChallenge,
						SolveTPMAuthnDeviceChallenge: dev.SolveTPMAuthnDeviceChallenge,
						GetDeviceOSType:              dev.GetDeviceOSType,
					}
				},
			),
			)
			require.NoError(t, err)

			stream, err := env.secretsScannerClient.ReportSecrets(ctx)
			require.NoError(t, err)

			err = ceremony.Run(ctx, clientStreamAdapter{stream: stream})
			if tt.failEarly {
				tt.assertErr(t, stream)
				return
			}
			require.NoError(t, err)

			err = stream.Send(accessgraphsecretsv1pb.ReportSecretsRequest_builder{
				PrivateKeys: accessgraphsecretsv1pb.ReportPrivateKeys_builder{
					Keys: tt.newPrivateKeys,
				}.Build(),
			}.Build())
			require.NoError(t, err)

			err = stream.CloseSend()
			require.NoError(t, err)
			tt.assertErr(t, stream)

			got := collectAll(t, ctx, env.accessGraphSecretsStorage.ListAllPrivateKeys)

			require.Empty(t,
				cmp.Diff(tt.expectedPrivateKeys, got,
					cmpopts.SortSlices(func(a, b *accessgraphsecretsv1pb.PrivateKey) bool {
						return a.GetMetadata().GetName() < b.GetMetadata().GetName()
					}),
					protocmp.Transform(),
					protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"),
				),
			)

			if tt.assertUsageReports != nil {
				tt.assertUsageReports(t, env.usageReporter)
			}
		})
	}

}

func collectAll[T any](t *testing.T, ctx context.Context, f func(ctx context.Context, pageSize int, pageToken string) ([]T, string, error)) []T {
	var out []T
	pageToken := ""
	for {
		items, next, err := f(ctx, 0, pageToken)
		require.NoError(t, err)
		out = append(out, items...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return out
}

func genBuiltinRoleAuthzContext(username, clusterName string, role types.SystemRole) *authz.Context {
	svcUser := fmt.Sprintf("%s.%s", username, clusterName)
	return &authz.Context{
		Identity: authz.BuiltinRole{
			Role:     role,
			Username: svcUser,
			Identity: tlsca.Identity{
				Username: svcUser,
			},
		},
		Checker: fakeAccessChecker{role: role},
	}
}

func genRemoteRoleAuthzContext(username, clusterName string, role types.SystemRole) *authz.Context {
	svcUser := fmt.Sprintf("%s.%s", username, clusterName)
	return &authz.Context{
		Identity: authz.BuiltinRole{
			Role:     role,
			Username: svcUser,
			Identity: tlsca.Identity{
				Username: svcUser,
			},
		},
		Checker: fakeAccessChecker{role: role},
	}
}

type fakeAccessChecker struct {
	services.AccessChecker
	role types.SystemRole
}

func (f fakeAccessChecker) HasRole(role string) bool {
	return string(f.role) == role
}

func newAuthorizedKey(t *testing.T, hostID string, fingerprint string) *accessgraphsecretsv1pb.AuthorizedKey {
	t.Helper()
	k, err := accessgraph.NewAuthorizedKey(
		accessgraphsecretsv1pb.AuthorizedKeySpec_builder{
			HostUser:       "user",
			HostId:         hostID,
			KeyFingerprint: fingerprint,
			KeyType:        "ssh-rsa",
		}.Build(),
	)
	require.NoError(t, err)
	return k
}

type clientStreamAdapter struct {
	stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient
}

func (a clientStreamAdapter) Send(pb *devicepb.AssertDeviceRequest) error {
	err := a.stream.Send(
		&accessgraphsecretsv1pb.ReportSecretsRequest{
			Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
				DeviceAssertion: pb,
			},
		},
	)
	return trace.Wrap(err)
}
func (a clientStreamAdapter) Recv() (*devicepb.AssertDeviceResponse, error) {
	resp, err := a.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp == nil || resp.GetDeviceAssertion() == nil {
		return nil, trace.BadParameter("assertion response payload required")
	}
	return resp.GetDeviceAssertion(), nil
}

func newPrivateKey(t *testing.T, fingerprint string) *accessgraphsecretsv1pb.PrivateKey {
	t.Helper()
	k, err := accessgraph.NewPrivateKey(
		accessgraphsecretsv1pb.PrivateKeySpec_builder{
			PublicKeyMode:        accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED,
			DeviceId:             "something",
			PublicKeyFingerprint: fingerprint,
		}.Build(),
	)
	require.NoError(t, err)
	return k
}

type fakeAnonymizer struct {
	utils.Anonymizer
}

func (f fakeAnonymizer) AnonymizeString(s string) string {
	return strings.ToUpper(s)
}
