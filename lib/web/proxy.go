package web

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/oxy/forward"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/route"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/agent"
)

type proxyHandler struct {
	host   string
	router route.Router
	auth   []utils.NetAddr
	id     int32
	cp     http.Handler
}

func newProxyHandler(cp http.Handler, auth []utils.NetAddr, host string) *proxyHandler {
	return &proxyHandler{
		router: route.New(),
		cp:     cp,
		host:   host,
		auth:   auth,
	}
}

func (p *proxyHandler) serveProxyRequest(prefix string, w http.ResponseWriter, r *http.Request) error {
	// TODO(klizhentas) cache connections per session and clean this up
	user, clt, err := authClient(p.auth, r)
	if err != nil {
		return fmt.Errorf("failed to auth: %v", err)
	}
	defer clt.Close()
	tun, err := clt.GetWebTun(prefix)
	if err != nil {
		return fmt.Errorf("web tunnel not found: %v", err)
	}
	agent, err := clt.GetAgent()
	if err != nil {
		return fmt.Errorf("failed to retrieve agent for tunnel: %v", err)
	}
	fwd, err := forward.New(
		forward.RoundTripper(&http.Transport{
			Dial: (&tunDialer{addr: tun.ProxyAddr, user: user, agent: agent}).Dial,
		}),
		forward.Logger(log.GetLogger()),
	)
	if err != nil {
		return fmt.Errorf("failed to create forwarder: %v", err)
	}
	url, err := url.ParseRequestURI(tun.TargetAddr)
	if err != nil {
		return fmt.Errorf("failed to parse request URI: %v", url)
	}
	r.URL = url
	fwd.ServeHTTP(w, r)
	return nil
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	prefix, found := findPrefix(r.Host, p.host)
	if !found || prefix == "cp" { // let control panel handle request
		p.cp.ServeHTTP(w, r)
		return
	}
	if err := p.serveProxyRequest(prefix, w, r); err != nil {
		log.Errorf("err: %v", err)
		// let the control panel to serve the request then
		// TODO(klizhentas): figure out the error handling policy better
		p.cp.ServeHTTP(w, r)
	}
}

func findPrefix(host, base string) (string, bool) {
	h := strings.Split(strings.ToLower(host), ":")[0]
	suffix := "." + base
	if strings.HasSuffix(h, suffix) {
		return strings.TrimSuffix(h, suffix), true
	}
	return "", false
}

type tunDialer struct {
	addr string
	user string

	sync.Mutex
	tun   *ssh.Client
	agent agent.Agent
}

func (t *tunDialer) getClient() (*ssh.Client, error) {
	t.Lock()
	defer t.Unlock()
	if t.tun != nil {
		return t.tun, nil
	}
	signers, err := t.agent.Signers()
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: t.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signers...)},
	}
	log.Infof("tunDialer.Dial(%v)", t.addr)
	client, err := ssh.Dial("tcp", t.addr, config)
	if err != nil {
		log.Infof("dial %v failed: %v", t.addr, err)
		return nil, err
	}
	t.tun = client
	return t.tun, nil
}

func (t *tunDialer) Dial(network, address string) (net.Conn, error) {
	c, err := t.getClient()
	if err != nil {
		return nil, err
	}
	return c.Dial(network, address)
}

func authClient(authSrv []utils.NetAddr, r *http.Request) (string, *auth.TunClient, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return "", nil, fmt.Errorf("no session cookie: %v", err)
	}
	d, err := DecodeCookie(cookie.Value)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode session cookie: %v", err)
	}
	method, err := auth.NewWebSessionAuth(d.User, []byte(d.SID))
	if err != nil {
		return "", nil, err
	}
	clt, err := auth.NewTunClient(authSrv[0], d.User, method)
	if err != nil {
		log.Errorf("failed to get tunnel client, err: %v", err)
		return "", nil, err
	}
	return d.User, clt, nil
}
