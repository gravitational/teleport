package clusters

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
)

type CLICommandProvider interface {
	GetCommand(cluster *Cluster, gateway *gateway.Gateway) (*string, error)
}

type dbcmdCLICommandProvider struct{}

func (d dbcmdCLICommandProvider) GetCommand(cluster *Cluster, gateway *gateway.Gateway) (*string, error) {
	routeToDb := tlsca.RouteToDatabase{
		ServiceName: gateway.TargetName,
		Protocol:    gateway.Protocol,
		Username:    gateway.TargetUser,
		Database:    gateway.TargetSubresourceName,
	}

	cmd, err := dbcmd.NewCmdBuilder(cluster.clusterClient, &cluster.status, &routeToDb,
		// TODO(ravicious): Pass the root cluster name here. GetActualName returns leaf name for leaf
		// clusters.
		//
		// At this point it doesn't matter though because this argument is used only for
		// generating correct CA paths. We use dbcmd.WithNoTLS here which means that the CA paths aren't
		// included in the returned CLI command.
		cluster.GetActualName(),
		dbcmd.WithLogger(gateway.Log),
		dbcmd.WithLocalProxy(gateway.LocalAddress, gateway.LocalPortInt(), ""),
		dbcmd.WithNoTLS(),
		dbcmd.WithTolerateMissingCLIClient(),
	).GetConnectCommandNoAbsPath()

	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmdString := strings.TrimSpace(fmt.Sprintf("%s %s", strings.Join(cmd.Env, " "), cmd.String()))

	return &cmdString, nil
}
