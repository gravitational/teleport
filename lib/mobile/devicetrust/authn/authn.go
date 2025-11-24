package authn

import (
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	"github.com/gravitational/teleport/lib/devicetrust/authn"
	"github.com/gravitational/teleport/lib/devicetrust/native"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.Component("dt", "authn"))

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

func (c *Ceremony) RunWeb(dc *DevicesClient, webToken *DeviceWebToken) (*DeviceConfirmationToken, error) {
	// TODO: Figure out how to deal with contexts in Swift.
	ctx := context.TODO()
	ct, err := c.ac.RunWeb(ctx, dc.dc, &devicepb.DeviceWebToken{
		Id:    webToken.TokenID,
		Token: webToken.Token,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &DeviceConfirmationToken{
		TokenID: ct.Id,
		Token:   ct.Token,
	}, nil
}

type DevicesClient struct {
	dc   devicepb.DeviceTrustServiceClient
	conn *grpc.ClientConn
}

type ClientConfig struct {
	// ProxyServer is the host of the proxy service, e.g., "teleport.example.com:3080"
	ProxyServer string
	Insecure    bool
}

func NewDevicesClient(cfg *ClientConfig) (*DevicesClient, error) {
	grpcConn, err := proxyinsecureclient.NewConnection(
		context.TODO(),
		proxyinsecureclient.ConnectionConfig{
			ProxyServer: cfg.ProxyServer,
			Insecure:    cfg.Insecure,
			Clock:       clockwork.NewRealClock(),
			Log:         logger,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &DevicesClient{
		dc:   devicepb.NewDeviceTrustServiceClient(grpcConn),
		conn: grpcConn,
	}, nil
}

func (dc *DevicesClient) Close() error {
	return trace.Wrap(dc.conn.Close())
}

type DeviceWebToken struct {
	TokenID string
	Token   string
}

type DeviceConfirmationToken struct {
	TokenID string
	Token   string
}
