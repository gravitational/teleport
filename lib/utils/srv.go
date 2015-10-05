package utils

import (
	"net"
	"net/http"
)

func StartHTTPServer(addr NetAddr, h http.Handler) error {
	if addr.Network == "tcp" {
		hsrv := &http.Server{
			Addr:    addr.Addr,
			Handler: h,
		}
		return hsrv.ListenAndServe()
	}
	l, err := net.Listen(addr.Network, addr.Addr)
	if err != nil {
		return err
	}
	hsrv := &http.Server{
		Handler: h,
	}
	return hsrv.Serve(l)
}
