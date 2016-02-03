package srv

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// proxySubsys is an SSH subsystem for easy proxyneling through proxy server
// This subsystem creates a new TCP connection and connects ssh channel
// with this connection
type proxySitesSubsys struct {
	srv *Server
}

func parseProxySitesSubsys(name string, srv *Server) (*proxySitesSubsys, error) {
	return &proxySitesSubsys{
		srv: srv,
	}, nil
}

func (t *proxySitesSubsys) String() string {
	return "proxySites()"
}

func (t *proxySitesSubsys) execute(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v execute()", ctx)
	sites := map[string]interface{}{}
	for _, s := range t.srv.proxyTun.GetSites() {
		servers, err := s.GetServers()
		if err != nil {
			panic(err.Error())
			return trace.Wrap(err)
		}
		sites[s.GetName()] = servers
	}

	data, err := json.Marshal(sites)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := ch.Write(data); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
