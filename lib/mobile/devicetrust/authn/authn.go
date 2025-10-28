package authn

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	// devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	// "github.com/gravitational/teleport/lib/devicetrust/native"
)

// type NativeCeremony interface {
// 	GetDeviceCredential() (*devicepb.DeviceCredential, error)
// 	CollectDeviceData(mode native.CollectDataMode) (*devicepb.DeviceCollectedData, error)
// 	SignChallenge(chal []byte) (sig []byte, err error)
// 	GetDeviceOSType() devicepb.OSType
// }

// type Ceremony struct {
// 	nc NativeCeremony
// }

// func NewCeremony(ceremony NativeCeremony)  {
// }

// func (c *Ceremony) RunWeb()

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
