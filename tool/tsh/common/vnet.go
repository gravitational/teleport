package common

import (
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/trace"
)

func onVNet(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(vnet.Run(cf.Context, tc))
}
