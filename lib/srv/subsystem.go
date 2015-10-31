package srv

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type subsys struct {
	Name string
}

type subsystem interface {
	execute(*ssh.ServerConn, ssh.Channel, *ssh.Request, *ctx) error
}

func parseSubsystemRequest(srv *Server, req *ssh.Request) (subsystem, error) {
	var s subsys
	if err := ssh.Unmarshal(req.Payload, &s); err != nil {
		return nil, fmt.Errorf("failed to parse subsystem request, error: %v", err)
	}
	if strings.HasPrefix(s.Name, "tun:") {
		return parseTunSubsys(s.Name)
	}
	if strings.HasPrefix(s.Name, "mux:") {
		return parseMuxSubsys(s.Name)
	}
	if strings.HasPrefix(s.Name, "join:") {
		return parseJoinSubsys(s.Name, srv)
	}
	if strings.HasPrefix(s.Name, "ls:") {
		return parseLSSubsys(s.Name)
	}
	if strings.HasPrefix(s.Name, "proxy:") && (srv.proxyMode) {
		return parseProxySubsys(s.Name, srv)
	}
	return nil, fmt.Errorf("unrecognized subsystem: %v", s.Name)
}
