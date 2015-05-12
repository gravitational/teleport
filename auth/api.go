package auth

import (
	"net"
	"net/http"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/memlog"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/oxy/trace"
	"github.com/gravitational/teleport/utils"
)

func StartHTTPServer(a string, srv *AuthServer) error {
	addr, err := utils.ParseAddr(a)
	if err != nil {
		return err
	}
	t, err := trace.New(
		NewAPIServer(srv, memlog.New()),
		log.GetLogger().Writer(log.SeverityInfo))
	if err != nil {
		return err
	}
	if addr.Network == "tcp" {
		hsrv := &http.Server{
			Addr:    addr.Addr,
			Handler: t,
		}
		return hsrv.ListenAndServe()
	}
	l, err := net.Listen(addr.Network, addr.Addr)
	if err != nil {
		return err
	}
	hsrv := &http.Server{
		Handler: t,
	}
	return hsrv.Serve(l)
}
