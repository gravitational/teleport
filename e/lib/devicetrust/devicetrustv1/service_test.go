package devicetrustv1_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

func TestService_authz(t *testing.T) {
	authorizer := &fakeAuthorizer{}
	env := testenv.MustNew(testenv.WithAuthorizer(authorizer))
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	tests := []struct {
		name      string
		checker   *fakeChecker
		rpc       func() error
		assertErr func(error) bool
	}{
		{
			name: "CreateDevice",
			checker: &fakeChecker{
				wantRule: "device", // TODO(codingllama): Pull from api/types.KindDevice
				wantVerb: types.VerbCreate,
			},
			rpc: func() error {
				_, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{})
				return err
			},
			assertErr: trace.IsBadParameter,
		},
		{
			name: "GetDevice",
			checker: &fakeChecker{
				wantRule: "device",
				wantVerb: types.VerbRead,
			},
			rpc: func() error {
				_, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
					DeviceId: "unknown",
				})
				return err
			},
			assertErr: trace.IsNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer.Checker = test.checker
			authorizer.authorizeCount = 0

			// Any "blessed" error OK, we expect the RPCs to fail after authorization.
			// It's simpler to test this way.
			if err := test.rpc(); !test.assertErr(err) {
				t.Fatalf("RPC assertErr failed, err=%v", err)
			}

			if got, want := authorizer.authorizeCount, 1; got != want {
				t.Errorf("Authorize count mismatch: got=%v, want=%v", got, want)
			}
			if got, want := test.checker.checkAccessToRuleCount, 1; got != want {
				t.Errorf("CheckAccessToRule count mismatch: got=%v, want=%v", got, want)
			}
		})
	}
}

type fakeAuthorizer struct {
	authorizeCount int
	Checker        services.AccessChecker
}

func (a *fakeAuthorizer) Authorize(ctx context.Context) (*auth.Context, error) {
	a.authorizeCount++

	user, err := types.NewUser("llama")
	if err != nil {
		return nil, err
	}

	return &auth.Context{
		User:    user,
		Checker: a.Checker,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker

	checkAccessToRuleCount int
	wantRule, wantVerb     string
}

func (c *fakeChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace string, rule string, verb string, silent bool) error {
	c.checkAccessToRuleCount++
	switch {
	case namespace != defaults.Namespace:
		return fmt.Errorf("unexpected namespace: %v", namespace)
	case rule != c.wantRule:
		return fmt.Errorf("unexpected rule=%q, want %q", rule, c.wantRule)
	case verb != c.wantVerb:
		return fmt.Errorf("unexpected verb=%q, want %q", verb, c.wantVerb)
	}
	return nil
}

func TestService_CreateDevice(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()
	devices := env.DevicesClient

	ctx := context.Background()
	tests := []struct {
		name string
		req  *devicepb.CreateDeviceRequest
	}{
		{
			name: "ok",
			req: &devicepb.CreateDeviceRequest{
				Device: &devicepb.Device{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "llama",
				},
			},
		},
		{
			name: "device and enroll token",
			req: &devicepb.CreateDeviceRequest{
				Device: &devicepb.Device{
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					AssetTag: "alpaca",
				},
				CreateEnrollToken: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := devices.CreateDevice(ctx, test.req)
			if err != nil {
				t.Fatalf("CreateDevice failed: %v", err)
			}

			// Assert that expected fields are present.
			if got.Id == "" {
				t.Fatal("CreateDevice returned device without ID")
			}
			if got.ApiVersion == "" {
				t.Error("CreateDevice returned device without ApiVersion")
			}
			if got.CreateTime == nil {
				t.Error("CreateDevice returned device without CreateTime")
			}
			if got.UpdateTime == nil {
				t.Error("CreateDevice returned device without CreateTime")
			}

			// Verify CreateDevice response.
			want := proto.Clone(test.req.Device).(*devicepb.Device)
			want.ApiVersion = got.ApiVersion
			want.Id = got.Id
			want.CreateTime = got.CreateTime
			want.UpdateTime = got.UpdateTime
			want.EnrollToken = got.EnrollToken
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Fatalf("CreateDevice mismatch (-want +got):\n%s", diff)
			}

			// Verify enrollment token.
			switch {
			case test.req.CreateEnrollToken:
				if got.EnrollToken.GetToken() == "" {
					t.Error("CreateDevice returned nil or empty enroll token, expected a non-empty token present")
				}
			case got.EnrollToken != nil:
				t.Errorf("CreateDevice returned an unexpected enroll token: %#v", got.EnrollToken)
			}
			// No other endpoints return the token, so blank it to make subsequent
			// comparisons easier.
			got.EnrollToken = nil

			// Verify that device is stored.
			stored, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: got.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			if diff := cmp.Diff(got, stored, protocmp.Transform()); diff != "" {
				t.Errorf("GetDevice mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
