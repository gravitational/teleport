package cp

import (
	"fmt"
	"net/http"

	"github.com/gravitational/teleport/utils"
)

// CPSrv implements Control Panel server
type CPServer struct {
	cfg Config
	h   http.Handler
}

type Config struct {
	AuthSrv []utils.NetAddr
	Host    string
}

func NewServer(cfg Config) (*CPServer, error) {
	if len(cfg.AuthSrv) == 0 {
		return nil, fmt.Errorf("need at least one auth server")
	}
	if cfg.Host == "" {
		return nil, fmt.Errorf("need an base host")
	}
	cp := newCPHandler(cfg.Host, cfg.AuthSrv)
	proxy := newProxyHandler(cp, cfg.AuthSrv, cfg.Host)
	return &CPServer{
		cfg: cfg,
		h:   proxy,
	}, nil
}

func (s *CPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.h.ServeHTTP(w, r)
}
