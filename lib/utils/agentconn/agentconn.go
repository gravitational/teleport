package agentconn

import (
	"fmt"
	"net"
	"os"
)

const SocketEnvironmentVariableName = "SSH_AUTH_SOCK"

func DialDefaultAgent() (net.Conn, error) {
	socket := os.Getenv(SocketEnvironmentVariableName)

	if socket == "" {
		return nil, fmt.Errorf("%s is not set")
	}

	return dialAgent(socket)
}

func DialAgent(socket string) (net.Conn, error) {
	return dialAgent(socket)
}
