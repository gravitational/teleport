package teleagent

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/teleport/lib/utils"

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

	srv.POST("/v1/login", srv.login)

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
	var args loginArgs
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	err = s.ag.Login(args.ProxyAddr, args.User, args.Password,
		args.HotpToken, args.TTL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Login error: " + err.Error()))
		return
	}

	w.Write([]byte(LoginSuccess))
}

type loginArgs struct {
	ProxyAddr string
	User      string
	Password  string
	HotpToken string
	TTL       time.Duration
}

const (
	DefaultAgentAPIAddress = "unix:///tmp/teleport.agent.api.sock"
	LoginSuccess           = "Logged in successfully"
)
