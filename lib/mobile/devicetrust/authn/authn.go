package authn

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	"github.com/gravitational/teleport/lib/devicetrust/authn"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

// NativeCeremony is a wrapper for [authn.Ceremony] that works with gomobile. gomobile doesn't
// support types defined outside of the package that gomobile is executed on, hence why all protobuf
// types are redefined within this package.
type NativeCeremony interface {
	GetDeviceCredential() (*DeviceCredential, error)
	CollectDeviceData() (*DeviceCollectedData, error)
	SignChallenge(chal []byte) (sig []byte, err error)
}

type DeviceCollectedData struct {
	SerialNumber    string
	ModelIdentifier string
	// Struct fields shouldn't start with an acronym due to a bug in gomobile.
	// https://github.com/golang/go/issues/32008
	VersionOS          string
	BuildOS            string
	SystemSerialNumber string
	// CollectTime and OSType are filled out in Go. [timestamppb.Timestamp] is not supported by
	// gomobile and [devicepb.OSType] would need to be redefined here as gomobile doesn't support
	// types defined outside of the mobile package.
}

type DeviceCredential struct {
	// "ID" would translate to a reserved Obj-C keyword, hence DeviceID.
	// https://cs.opensource.google/go/x/mobile/+/master:bind/genobjc.go;l=1415-1419;drc=7c4916698cc93475ebfea76748ee0faba2deb2a5
	DeviceID     string
	PublicKeyDER []byte
}

type Ceremony struct {
	ac *authn.Ceremony
}

func NewCeremony(nc NativeCeremony) *Ceremony {
	return &Ceremony{
		ac: &authn.Ceremony{
			GetDeviceCredential: func() (*devicepb.DeviceCredential, error) {
				dc, err := nc.GetDeviceCredential()
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return &devicepb.DeviceCredential{
					Id:           dc.DeviceID,
					PublicKeyDer: dc.PublicKeyDER,
				}, nil
			},
			CollectDeviceData: func(native.CollectDataMode) (*devicepb.DeviceCollectedData, error) {
				// CollectDataMode applies only to Linux, so it is ignored here.
				cd, err := nc.CollectDeviceData()
				if err != nil {
					return nil, trace.Wrap(err)
				}

				return &devicepb.DeviceCollectedData{
					CollectTime:        timestamppb.Now(),
					OsType:             devicepb.OSType_OS_TYPE_MACOS,
					SerialNumber:       cd.SerialNumber,
					ModelIdentifier:    cd.ModelIdentifier,
					OsVersion:          cd.VersionOS,
					OsBuild:            cd.BuildOS,
					SystemSerialNumber: cd.SystemSerialNumber,
				}, nil
			},
			SignChallenge: nc.SignChallenge,
			// Not supported on iOS.
			SolveTPMAuthnDeviceChallenge: nil,
			GetDeviceOSType: func() devicepb.OSType {
				return devicepb.OSType_OS_TYPE_MACOS
			},
		},
	}
}

func (c *Ceremony) RunWeb() error {
	dc, err := c.ac.GetDeviceCredential()
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Got device credential in Go: id=%s der=%s\n", dc.Id, dc.PublicKeyDer)
	return nil
}

type Greeter struct {
}

func NewGreeter() *Greeter {
	return &Greeter{}
}

func (g *Greeter) Greet(name string) string {
	return "Hello " + name
}

func (g *Greeter) Ping() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcConn, err := proxyinsecureclient.NewConnection(
		ctx,
		proxyinsecureclient.ConnectionConfig{
			ProxyServer: "teleport-mbp.ocelot-paradise.ts.net:3030",
			Clock:       clockwork.NewRealClock(),
			Insecure:    false,
			Log:         slog.Default(),
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	client := accessgraphsecretsv1pb.NewSecretsScannerServiceClient(grpcConn)
	_, err = client.Ping(ctx, &accessgraphsecretsv1pb.PingRequest{})
	return trace.Wrap(err)
}
