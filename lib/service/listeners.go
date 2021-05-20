/*
Copyright 2020 Gravitational, Inc.

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

package service

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// listenerType identifies different registered listeners in
// process.registeredListeners.
type listenerType string

var (
	listenerAuthSSH    = listenerType(teleport.ComponentAuth)
	listenerNodeSSH    = listenerType(teleport.ComponentNode)
	listenerProxySSH   = listenerType(teleport.Component(teleport.ComponentProxy, "ssh"))
	listenerDiagnostic = listenerType(teleport.ComponentDiagnostic)
	listenerProxyKube  = listenerType(teleport.Component(teleport.ComponentProxy, "kube"))
	listenerKube       = listenerType(teleport.ComponentKube)
	// Proxy can use the same listener for tunnels and web interface
	// (multiplexing the requests).
	listenerProxyTunnelAndWeb = listenerType(teleport.Component(teleport.ComponentProxy, "tunnel", "web"))
	listenerProxyWeb          = listenerType(teleport.Component(teleport.ComponentProxy, "web"))
	listenerProxyTunnel       = listenerType(teleport.Component(teleport.ComponentProxy, "tunnel"))
	listenerProxyMySQL        = listenerType(teleport.Component(teleport.ComponentProxy, "mysql"))
)

// AuthSSHAddr returns auth server SSH endpoint, if configured and started.
func (process *TeleportProcess) AuthSSHAddr() (*utils.NetAddr, error) {
	return process.registeredListenerAddr(listenerAuthSSH)
}

// NodeSSHAddr returns the node SSH endpoint, if configured and started.
func (process *TeleportProcess) NodeSSHAddr() (*utils.NetAddr, error) {
	return process.registeredListenerAddr(listenerNodeSSH)
}

// ProxySSHAddr returns the proxy SSH endpoint, if configured and started.
func (process *TeleportProcess) ProxySSHAddr() (*utils.NetAddr, error) {
	return process.registeredListenerAddr(listenerProxySSH)
}

// DiagnosticAddr returns the diagnostic endpoint, if configured and started.
func (process *TeleportProcess) DiagnosticAddr() (*utils.NetAddr, error) {
	return process.registeredListenerAddr(listenerDiagnostic)
}

// ProxyKubeAddr returns the proxy kubernetes endpoint, if configured and
// started.
func (process *TeleportProcess) ProxyKubeAddr() (*utils.NetAddr, error) {
	return process.registeredListenerAddr(listenerProxyKube)
}

// ProxyWebAddr returns the proxy web interface endpoint, if configured and
// started.
func (process *TeleportProcess) ProxyWebAddr() (*utils.NetAddr, error) {
	addr, err := process.registeredListenerAddr(listenerProxyTunnelAndWeb)
	if err == nil {
		return addr, nil
	}
	return process.registeredListenerAddr(listenerProxyWeb)
}

// ProxyTunnelAddr returns the proxy reverse tunnel endpoint, if configured and
// started.
func (process *TeleportProcess) ProxyTunnelAddr() (*utils.NetAddr, error) {
	addr, err := process.registeredListenerAddr(listenerProxyTunnelAndWeb)
	if err == nil {
		return addr, nil
	}
	return process.registeredListenerAddr(listenerProxyTunnel)
}

func (process *TeleportProcess) registeredListenerAddr(typ listenerType) (*utils.NetAddr, error) {
	process.Lock()
	defer process.Unlock()

	var matched []registeredListener
	for _, l := range process.registeredListeners {
		if l.typ == typ {
			matched = append(matched, l)
		}
	}
	switch len(matched) {
	case 0:
		return nil, trace.NotFound("no registered address for type %q", typ)
	case 1:
		return utils.ParseAddr(matched[0].listener.Addr().String())
	default:
		return nil, trace.NotFound("multiple registered listeners found for type %q", typ)
	}
}
