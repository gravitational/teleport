package pam

import (
	"net"

	"github.com/gravitational/teleport/lib/service/servicecfg"
)

type Config struct {
	*servicecfg.PAMConfig
	TTYName    string
	RemoteAddr net.Addr
}
