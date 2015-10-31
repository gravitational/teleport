package teleagent

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/form"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
)

type AgentAPIServer struct {
	httprouter.Router
	ag *TeleAgent
}

func NewAgentAPIServer(ag *TeleAgent) *AgentAPIServer {
	srv := AgentAPIServer{}
	srv.ag = ag
	srv.Router = *httprouter.New()

	srv.POST("/login", srv.login)

	return &srv
}

func (s *AgentAPIServer) Start(apiAddr string) error {
	addr, err := utils.ParseAddr(apiAddr)

	l, err := net.Listen(addr.Network, addr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}

	hsrv := &http.Server{
		Handler: s,
	}

	return hsrv.Serve(l)
}

func (s *AgentAPIServer) login(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var proxyAddr, user, pass, hotpToken, ttlJSON string

	err := form.Parse(r,
		form.String("proxyAddr", &proxyAddr, form.Required()),
		form.String("user", &user, form.Required()),
		form.String("password", &pass, form.Required()),
		form.String("hotpToken", &hotpToken, form.Required()),
		form.String("ttl", &ttlJSON, form.Required()),
	)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	var ttl time.Duration
	if err != json.Unmarshal([]byte(ttlJSON), &ttl) {
		w.Write([]byte(err.Error()))
		return
	}

	err = s.ag.Login(proxyAddr, user, pass, hotpToken, ttl)
	if err != nil {
		w.Write([]byte("Login error: " + err.Error()))
		return
	}

	w.Write([]byte("Logged in successfully"))
}

const (
	DefaultAgentAPIAddress = "unix:///tmp/teleport.agent.api.sock"
)
