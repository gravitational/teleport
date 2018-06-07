// +build !windows

package agentconn

import "net"

func dialAgent(socket string) (net.Conn, error) {
	return net.Dial("unix", socket)
}
