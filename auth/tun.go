package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/agent"
)

type TunServer struct {
	certChecker ssh.CertChecker
	a           *AuthServer
	l           net.Listener
	srv         *sshutils.Server
	hostSigner  ssh.Signer
	authAddr    utils.NetAddr
}

type ServerOption func(s *TunServer) error

// New returns an unstarted server
func NewTunServer(addr utils.NetAddr, hostSigners []ssh.Signer,
	authAddr utils.NetAddr, a *AuthServer,
	opts ...ServerOption) (*TunServer, error) {

	srv := &TunServer{
		a:        a,
		authAddr: authAddr,
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
		sshutils.AuthMethods{
			Password:  srv.passwordAuth,
			PublicKey: srv.keyAuth,
		})
	if err != nil {
		return nil, err
	}
	srv.certChecker = ssh.CertChecker{IsAuthority: srv.isAuthority}
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
	case ReqDirectTCPIP:
		if !s.haveExt(sconn, ExtHost, ExtWebSession, ExtWebPassword) {
			nch.Reject(
				ssh.UnknownChannelType,
				fmt.Sprintf("register clients can not TCPIP: %v", cht))
			return
		}
		req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
		if err != nil {
			log.Errorf("failed to parse request data: %v, err: %v",
				string(nch.ExtraData()), err)
			nch.Reject(ssh.UnknownChannelType,
				"failed to parse direct-tcpip request")
		}
		sshCh, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleDirectTCPIPRequest(sconn, sshCh, req)
	case ReqWebSessionAgent:
		// this is a protective measure, so web requests can be only done
		// if have session ready
		if !s.haveExt(sconn, ExtWebSession) {
			nch.Reject(
				ssh.UnknownChannelType,
				fmt.Sprintf("don't have web session for: %v", cht))
			return
		}
		ch, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleWebAgentRequest(sconn, ch)
	case ReqProvision:
		if !s.haveExt(sconn, ExtToken) {
			nch.Reject(
				ssh.UnknownChannelType,
				fmt.Sprintf("don't have token for: %v", cht))
			return
		}
		ch, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleProvisionRequest(sconn, ch)
	case ReqNewAuth:
		if !s.haveExt(sconn, ExtToken) {
			nch.Reject(
				ssh.UnknownChannelType,
				fmt.Sprintf("don't have token for: %v", cht))
			return
		}
		ch, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleNewAuthRequest(sconn, ch)

	default:
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf(
			"unknown channel type: %v", cht))
	}
}

// isAuthority is called during checking the client key, to see if the signing
// key is the real CA authority key.
func (s *TunServer) isAuthority(auth ssh.PublicKey) bool {
	key, err := s.a.GetHostCAPub()
	if err != nil {
		log.Errorf("failed to retrieve user authority key, err: %v", err)
		return false
	}
	cert, _, _, _, err := ssh.ParseAuthorizedKey(key)
	if err != nil {
		log.Errorf("failed to parse CA cert '%v', err: %v", string(key), err)
		return false
	}

	if !sshutils.KeysEqual(cert, auth) {
		log.Warningf("authority signature check failed, signing keys mismatch")
		return false
	}
	return true
}

func (s *TunServer) haveExt(sconn *ssh.ServerConn, ext ...string) bool {
	if sconn.Permissions == nil {
		return false
	}
	for _, e := range ext {
		if sconn.Permissions.Extensions[e] != "" {
			return true
		}
	}
	return true
}

func (s *TunServer) handleWebAgentRequest(sconn *ssh.ServerConn, ch ssh.Channel) {
	defer ch.Close()
	a := agent.NewKeyring()
	log.Infof("handleWebAgentRequest start for %v", sconn.RemoteAddr())

	sessionID := session.SecureID(sconn.Permissions.Extensions[ExtWebSession])

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
	addedKey := agent.AddedKey{
		PrivateKey:       priv,
		Certificate:      cert,
		Comment:          "web-session@teleport",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}
	if err := a.Add(addedKey); err != nil {
		log.Errorf("failed to add: %v", err)
		return
	}
	if err := agent.ServeAgent(a, ch); err != nil {
		log.Errorf("Serve agent err: %v", err)
	}
}

// this direct tcp-ip request ignores port and host requested by client
// and always forwards it to the local auth server listening on local socket
func (s *TunServer) handleDirectTCPIPRequest(sconn *ssh.ServerConn, ch ssh.Channel, req *sshutils.DirectTCPIPReq) {
	defer ch.Close()

	log.Infof("handleDirectTCPIPRequest start to %v:%d", req.Host, req.Port)
	log.Infof("opened direct-tcpip channel to server: %v", req)
	log.Infof("connecting to %v", s.authAddr)

	conn, err := net.Dial(s.authAddr.Network, s.authAddr.Addr)
	if err != nil {
		log.Infof("%v failed to connect to: %v, err: %v", s.authAddr.Addr, err)
		return
	}
	defer conn.Close()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(ch, conn)
		log.Infof("conn to channel copy closed, bytes transferred: %v, err: %v", written, err)
		ch.Close()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(conn, ch)
		log.Infof("channel to conn copy closed, bytes transferred: %v, err: %v", written, err)
		conn.Close() //  it is important to close this connection, otherwise this goroutine will hang forever
		// because the copoy from conn to ch can be never closed unless the connection is closed
	}()
	wg.Wait()
	log.Infof("direct-tcp closed")
}

func (s *TunServer) handleProvisionRequest(sconn *ssh.ServerConn, ch ssh.Channel) {
	defer ch.Close()

	password := []byte(sconn.Permissions.Extensions[ExtToken])

	var ab *authBucket
	if err := json.Unmarshal(password, &ab); err != nil {
		log.Errorf("token error: %v", err)
		return
	}

	k, pub, err := s.a.GenerateKeyPair("")
	if err != nil {
		log.Errorf("gen key pair error: %v", err)
		return
	}
	c, err := s.a.GenerateHostCert(pub, ab.User, ab.User, 0)
	if err != nil {
		log.Errorf("gen cert error: %v", err)
		return
	}

	keys := &PackedKeys{
		Key:  k,
		Cert: c,
	}
	data, err := json.Marshal(keys)
	if err != nil {
		log.Errorf("gen marshal error: %v", err)
		return
	}

	if _, err := io.Copy(ch.Stderr(), bytes.NewReader(data)); err != nil {
		log.Errorf("key transfer error: %v", err)
		return
	}

	log.Infof("keys for %v transferred successfully", ab.User)
	if err := s.a.DeleteToken(string(ab.Pass)); err != nil {
		log.Errorf("failed to delete token: %v", err)
	}
}

func (s *TunServer) handleNewAuthRequest(sconn *ssh.ServerConn, ch ssh.Channel) {
	defer ch.Close()

	log.Infof("Registering new auth server")

	password := []byte(sconn.Permissions.Extensions[ExtToken])

	var ab *authBucket
	if err := json.Unmarshal(password, &ab); err != nil {
		log.Errorf("token error: %v", err)
		return
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, ch.Stderr()); err != nil {
		log.Errorf("failed to read remote key from channel: %v", err)
		return
	}

	var remoteKey encryptor.Key
	if err := json.Unmarshal(buf.Bytes(), &remoteKey); err != nil {
		log.Errorf("key unmarshal error: %v", err)
		return
	}

	if err := s.a.AddSealKey(remoteKey); err != nil {
		log.Errorf("Can't add remote key to backend: %v", err)
		return
	}

	myKey, err := s.a.GetSignKey()
	if err != nil {
		log.Errorf("can't get backend sign key: %v", err)
		return
	}
	myKey = myKey.Public()

	data, err := json.Marshal(myKey)
	if err != nil {
		log.Errorf("gen marshal error: %v", err)
		return
	}

	if _, err := io.Copy(ch.Stderr(), bytes.NewReader(data)); err != nil {
		log.Errorf("key transfer error: %v", err)
		return
	}

	if err := ch.CloseWrite(); err != nil {
		log.Errorf("Can't close write: &v", err)
		return
	}

	log.Infof("keys for %v transferred successfully", ab.User)
	if err := s.a.DeleteToken(string(ab.Pass)); err != nil {
		log.Errorf("failed to delete token: %v", err)
	}
}

func (s *TunServer) keyAuth(
	conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	cid := fmt.Sprintf(
		"conn(%v->%v, user=%v)", conn.RemoteAddr(),
		conn.LocalAddr(), conn.User())

	log.Infof("%v auth attempt with key %v", cid, key.Type())

	err := s.certChecker.CheckHostKey(conn.User(), conn.RemoteAddr(), key)
	if err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: failed auth user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, err
	}

	perms := &ssh.Permissions{
		Extensions: map[string]string{
			ExtHost: conn.User(),
		},
	}

	return perms, nil
}

func (s *TunServer) passwordAuth(
	conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	var ab *authBucket
	if err := json.Unmarshal(password, &ab); err != nil {
		return nil, err
	}
	log.Infof("got authentication attempt for user '%v' type '%v'", conn.User(), ab.Type)
	switch ab.Type {
	case "password":
		if err := s.a.CheckPassword(conn.User(), ab.Pass); err != nil {
			log.Errorf("Password auth error: %v", err)
			return nil, err
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebPassword: "<password>",
			},
		}
		log.Infof("password authenticated user: '%v'", conn.User())
		return perms, nil
	case "session":
		// we use extra permissions mechanism to keep the connection data
		// after authorization, in this case the session
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtWebSession: string(ab.Pass),
			},
		}
		if _, err := s.a.GetWebSession(conn.User(), session.SecureID(ab.Pass)); err != nil {
			log.Errorf("session resume error: %v", err)
			return nil, err
		}
		log.Infof("session authenticated user: '%v'", conn.User())
		return perms, nil
	case "provision-token":
		if err := s.a.ValidateToken(string(ab.Pass), ab.User); err != nil {
			err := fmt.Errorf("%v token validation error: %v", ab.User, err)
			log.Errorf("%v", err)
			return nil, err
		}
		perms := &ssh.Permissions{
			Extensions: map[string]string{
				ExtToken: string(password),
			}}
		log.Infof("session authenticated prov. token: '%v'", conn.User())
		return perms, nil
	default:
		log.Errorf("unsupported auth method: '%v'", ab.Type)
		return nil, fmt.Errorf("unsupported auth method: '%v'", ab.Type)
	}
}

// authBucket uses password to transport app-specific user name and
// auth-type in addition to the password to support auth
type authBucket struct {
	User string `json:"user"`
	Type string `json:"type"`
	Pass []byte `json:"pass"`
}

func NewTokenAuth(fqdn, token string) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthToken,
		User: fqdn,
		Pass: []byte(token),
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

func NewWebSessionAuth(user string, session []byte) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthWebSession,
		User: user,
		Pass: session,
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

func NewWebPasswordAuth(user string, password []byte) ([]ssh.AuthMethod, error) {
	data, err := json.Marshal(authBucket{
		Type: AuthWebPassword,
		User: user,
		Pass: password,
	})
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.Password(string(data))}, nil
}

func NewHostAuth(key, cert []byte) ([]ssh.AuthMethod, error) {
	signer, err := sshutils.NewSigner(key, cert)
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

type TunClient struct {
	Client
	dialer *TunDialer
	tr     *http.Transport
}

func NewTunClient(addr utils.NetAddr, user string, auth []ssh.AuthMethod) (*TunClient, error) {
	tc := &TunClient{
		dialer: &TunDialer{auth: auth, addr: addr, user: user},
	}
	tr := &http.Transport{
		Dial: tc.dialer.Dial,
	}
	clt, err := NewClient(
		"http://stub:0",
		roundtrip.HTTPClient(&http.Client{
			Transport: tr,
		}))
	if err != nil {
		return nil, err
	}
	tc.Client = *clt
	tc.tr = tr
	return tc, nil
}

func (c *TunClient) GetAgent() (agent.Agent, error) {
	return c.dialer.GetAgent()
}

func (c *TunClient) Close() error {
	c.tr.CloseIdleConnections()
	return c.dialer.Close()
}

func (c *TunClient) GetDialer() AccessPointDialer {
	return func() (net.Conn, error) {
		return c.dialer.Dial(c.dialer.addr.Network, "accesspoint:0")
	}
}

type TunDialer struct {
	sync.Mutex
	auth []ssh.AuthMethod
	user string
	tun  *ssh.Client
	addr utils.NetAddr
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
	ch, _, err := t.tun.OpenChannel(ReqWebSessionAgent, nil)
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

	config := &ssh.ClientConfig{
		User: t.user,
		Auth: t.auth,
	}
	client, err := ssh.Dial(t.addr.Network, t.addr.Addr, config)
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
	ReqWebSessionAgent = "web-session-agent@teleport"
	ReqProvision       = "provision@teleport"
	ReqDirectTCPIP     = "direct-tcpip"
	ReqNewAuth         = "new-auth@teleport"

	ExtWebSession  = "web-session@teleport"
	ExtWebPassword = "web-password@teleport"
	ExtToken       = "provision@teleport"
	ExtHost        = "host@teleport"

	AuthWebPassword = "password"
	AuthWebSession  = "session"
	AuthToken       = "provision-token"
)

// AccessPointDialer dials to auth access point  remote HTTP api
type AccessPointDialer func() (net.Conn, error)
