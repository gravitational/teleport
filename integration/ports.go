/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"fmt"
	"net"
	"strconv"

	"github.com/gravitational/teleport/lib/utils"
)

// ports contains tcp ports allocated for all integration tests.
var ports utils.PortList

func init() {
	// Allocate tcp ports for all integration tests. 5000 should be plenty.
	var err error
	ports, err = utils.GetFreeTCPPorts(5000, utils.PortStartingNumber)
	if err != nil {
		panic(fmt.Sprintf("failed to allocate tcp ports for tests: %v", err))
	}
}

func newInstancePort() *InstancePort {
	i := ports.PopInt()
	p := InstancePort(i)
	return &p

}

type InstancePort int

func (p *InstancePort) String() string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(int(*p))
}

func singleProxyPortSetup() *InstancePorts {
	v := newInstancePort()
	return &InstancePorts{
		Web:               v,
		SSHProxy:          v,
		ReverseTunnel:     v,
		MySQL:             v,
		SSH:               newInstancePort(),
		Auth:              newInstancePort(),
		isSinglePortSetup: true,
	}
}
func standardPortSetup() *InstancePorts {
	return &InstancePorts{
		Web:           newInstancePort(),
		SSH:           newInstancePort(),
		Auth:          newInstancePort(),
		SSHProxy:      newInstancePort(),
		ReverseTunnel: newInstancePort(),
		MySQL:         newInstancePort(),
	}
}

func webReverseTunnelMuxPortSetup() *InstancePorts {
	v := newInstancePort()
	return &InstancePorts{
		Web:           v,
		ReverseTunnel: v,
		SSH:           newInstancePort(),
		SSHProxy:      newInstancePort(),
		MySQL:         newInstancePort(),
		Auth:          newInstancePort(),
	}
}

type InstancePorts struct {
	Host string
	Web  *InstancePort
	// SSH is an instance of SSH server Port.
	SSH *InstancePort
	// SSHProxy is Teleport SSH Proxy Port.
	SSHProxy      *InstancePort
	Auth          *InstancePort
	ReverseTunnel *InstancePort
	MySQL         *InstancePort

	isSinglePortSetup bool
}

func (i *InstancePorts) GetPortSSHInt() int           { return int(*i.SSH) }
func (i *InstancePorts) GetPortSSH() string           { return i.SSH.String() }
func (i *InstancePorts) GetPortAuth() string          { return i.Auth.String() }
func (i *InstancePorts) GetPortProxy() string         { return i.SSHProxy.String() }
func (i *InstancePorts) GetPortWeb() string           { return i.Web.String() }
func (i *InstancePorts) GetPortMySQL() string         { return i.MySQL.String() }
func (i *InstancePorts) GetPortReverseTunnel() string { return i.ReverseTunnel.String() }

func (i *InstancePorts) GetSSHAddr() string {
	if i.SSH == nil {
		return ""
	}
	return net.JoinHostPort(i.Host, i.GetPortSSH())
}

func (i *InstancePorts) GetAuthAddr() string {
	if i.Auth == nil {
		return ""
	}
	return net.JoinHostPort(i.Host, i.GetPortAuth())
}

func (i *InstancePorts) GetProxyAddr() string {
	if i.SSHProxy == nil {
		return ""
	}
	return net.JoinHostPort(i.Host, i.GetPortProxy())
}

func (i *InstancePorts) GetWebAddr() string {
	if i.Web == nil {
		return ""
	}
	return net.JoinHostPort(i.Host, i.GetPortWeb())
}

func (i *InstancePorts) GetMySQLAddr() string {
	if i.MySQL == nil {
		return ""
	}
	return net.JoinHostPort(i.Host, i.GetPortMySQL())
}

func (i *InstancePorts) GetReverseTunnelAddr() string {
	if i.ReverseTunnel == nil {
		return ""
	}
	return net.JoinHostPort(i.Host, i.GetPortReverseTunnel())
}
