package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/gravitational/teleport/sshutils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/agent"
)

type TunServer struct {
	certChecker ssh.CertChecker
	a           *AuthServer
	l           net.Listener
	srv         *sshutils.Server
	hostSigner  ssh.Signer
	authAddr    string
}

type ServerOption func(s *TunServer) error

// New returns an unstarted server
func NewTunServer(addr sshutils.Addr, hostSigners []ssh.Signer, authURL string, a *AuthServer, opts ...ServerOption) (*TunServer, error) {
	u, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}
	srv := &TunServer{
		a:        a,
		authAddr: u.Host,
	}
	for _, o := range opts {
		if err := o(srv); err != nil {
			return nil, err
		}
	}
	s, err := sshutils.NewServer(
		addr,
		srv,
		hostSigners,
		sshutils.AuthMethods{Password: srv.passwordAuth})
	if err != nil {
		return nil, err
	}
	srv.srv = s
	return srv, nil
}

func (s *TunServer) Addr() string {
	return s.srv.Addr()
}

func (s *TunServer) Start() error {
	return s.srv.Start()
}

func (s *TunServer) Close() error {
	return s.srv.Close()
}

func (s *TunServer) HandleNewChan(sconn *ssh.ServerConn, nch ssh.NewChannel) {
	log.Infof("got new channel request: %v", nch.ChannelType())
	cht := nch.ChannelType()
	switch cht {
	case "direct-tcpip":
		req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
		if err != nil {
			log.Errorf("failed to parse request data: %v, err: %v", string(nch.ExtraData()), err)
			nch.Reject(ssh.UnknownChannelType, "failed to parse direct-tcpip request")
		}
		sshCh, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleDirectTCPIPRequest(sconn, sshCh, req)
	case WebSessionAgentRequest:
		if !s.haveSession(sconn) {
			nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("don't have web session for: %v", cht))
			return
		}
		ch, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleWebAgentRequest(sconn, ch)
	default:
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", cht))
	}
}

func (s *TunServer) haveSession(sconn *ssh.ServerConn) bool {
	return sconn.Permissions != nil && sconn.Permissions.Extensions[SessionExt] != ""
}

func (s *TunServer) handleWebAgentRequest(sconn *ssh.ServerConn, ch ssh.Channel) {
	defer ch.Close()
	a := agent.NewKeyring()
	sessionID := session.SecureID(sconn.Permissions.Extensions[SessionExt])

	ws, err := s.a.GetWebSession(sconn.User(), sessionID)
	if err != nil {
		log.Errorf("session error: %v", err)
		return
	}

	priv, err := ssh.ParseRawPrivateKey(ws.WS.Priv)
	if err != nil {
		log.Errorf("session error: %v", err)
		return
	}

	pub, _, _, _, err := ssh.ParseAuthorizedKey(ws.WS.Pub)
	if err != nil {
		log.Errorf("session error: %v", err)
		return
	}

	cert, ok := pub.(*ssh.Certificate)
	if !ok {
		log.Errorf("session error, not a cert: %T", pub)
		return
	}
	if err := a.Add(priv, cert, "web-session@teleport"); err != nil {
		log.Errorf("failed to add: %v", err)
		return
	}
	if err := agent.ServeAgent(a, ch); err != nil {
		log.Errorf("Serve agent err: %v", err)
	}
}

func (s *TunServer) handleDirectTCPIPRequest(sconn *ssh.ServerConn, ch ssh.Channel, req *sshutils.DirectTCPIPReq) {
	defer ch.Close()

	log.Infof("opened direct-tcpip channel: %v", req)
	addr := fmt.Sprintf("%v:%d", req.Host, req.Port)
	log.Infof("connecting to %v", s.authAddr)
	conn, err := net.Dial("tcp", s.authAddr)
	if err != nil {
		log.Infof("failed to connect to: %v, err: %v", s.authAddr, addr)
		return
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(ch, conn)
		log.Infof("conn to channel copy closed, bytes transferred: %v, err: %v", written, err)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(conn, ch)
		log.Infof("channel to conn copy closed, bytes transferred: %v, err: %v", written, err)
	}()
	wg.Wait()
	log.Infof("direct-tcp closed")
}

func (s *TunServer) passwordAuth(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	var am *AuthMethod
	if err := json.Unmarshal(password, &am); err != nil {
		return nil, err
	}
	log.Infof("got authentication attempt for user '%v' type '%v'", conn.User(), am.Type)
	switch am.Type {
	case "password":
		if err := s.a.CheckPassword(conn.User(), am.Pass); err != nil {
			log.Errorf("Password auth error: %v", err)
			return nil, err
		}
		log.Infof("password authenticated user: '%v'", conn.User())
		return nil, nil
	case "session":
		perms := &ssh.Permissions{Extensions: map[string]string{"session@teleport": string(am.Pass)}}
		if _, err := s.a.GetWebSession(conn.User(), session.SecureID(am.Pass)); err != nil {
			log.Errorf("session resume error: %v", err)
			return nil, err
		}
		log.Infof("session authenticated user: '%v'", conn.User())
		return perms, nil
	default:
		log.Errorf("unsupported auth method: '%v'", am.Type)
		return nil, fmt.Errorf("unsupported auth method: '%v'", am.Type)
	}
}

type AuthMethod struct {
	User string
	Type string
	Pass []byte
}

type TunClient struct {
	Client
	dialer *TunDialer
}

func NewTunClient(addr string, auth AuthMethod) (*TunClient, error) {
	tc := &TunClient{
		dialer: &TunDialer{Auth: auth, Addr: addr},
	}
	tc.Client = Client{
		addr: "http://addr",
		client: &http.Client{
			Transport: &http.Transport{
				Dial: tc.dialer.Dial,
			},
		},
	}
	return tc, nil
}

func (c *TunClient) GetAgent() (agent.Agent, error) {
	return c.dialer.GetAgent()
}

func (c *TunClient) Close() error {
	return c.dialer.Close()
}

type TunDialer struct {
	sync.Mutex
	Addr string
	Auth AuthMethod

	tun *ssh.Client
}

func (t *TunDialer) Close() error {
	if t.tun != nil {
		return t.tun.Close()
	}
	return nil
}

func (t *TunDialer) GetAgent() (agent.Agent, error) {
	_, err := t.getClient() // we need an established connection first
	if err != nil {
		return nil, err
	}
	ch, _, err := t.tun.OpenChannel(WebSessionAgentRequest, nil)
	if err != nil {
		return nil, err
	}
	log.Infof("opened agent channel")
	return agent.NewClient(ch), nil
}

func (t *TunDialer) getClient() (*ssh.Client, error) {
	t.Lock()
	defer t.Unlock()
	if t.tun != nil {
		return t.tun, nil
	}
	bytes, err := json.Marshal(t.Auth)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: t.Auth.User,
		Auth: []ssh.AuthMethod{ssh.Password(string(bytes))},
	}
	client, err := ssh.Dial("tcp", t.Addr, config)
	if err != nil {
		return nil, err
	}
	t.tun = client
	return t.tun, nil
}

func (t *TunDialer) Dial(network, address string) (net.Conn, error) {
	c, err := t.getClient()
	if err != nil {
		return nil, err
	}
	return c.Dial(network, address)
}

const (
	WebSessionAgentRequest = "web-session-agent@teleport"
	SessionExt             = "session@teleport"
)
