package web

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/ttlmap"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type Cookie struct {
	User string
	SID  string
}

func EncodeCookie(user, sid string) (string, error) {
	bytes, err := json.Marshal(Cookie{User: user, SID: sid})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func DecodeCookie(b string) (*Cookie, error) {
	bytes, err := hex.DecodeString(b)
	if err != nil {
		return nil, err
	}
	var c *Cookie
	if err := json.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}
	return c, nil
}

type Context interface {
	io.Closer
	ConnectUpstream(addr string) (*sshutils.Upstream, error)
	GetAuthMethods() ([]ssh.AuthMethod, error)
	GetWebSID() string
	GetUser() string
	GetClient() auth.ClientI
}

type LocalContext struct {
	sid  string
	user string
	clt  *auth.TunClient
}

func (c *LocalContext) GetClient() auth.ClientI {
	return c.clt
}

func (c *LocalContext) GetUser() string {
	return c.user
}

func (c *LocalContext) GetWebSID() string {
	return c.sid
}

func (c *LocalContext) GetAuthMethods() ([]ssh.AuthMethod, error) {
	a, err := c.clt.GetAgent()
	if err != nil {
		log.Errorf("failed to get agent: %v", err)
		return nil, err
	}
	signers, err := a.Signers()
	if err != nil {
		log.Errorf("failed to get signers: %v", err)
		return nil, err
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signers...)}, nil
}

func (c *LocalContext) Close() error {
	if c.clt != nil {
		return c.clt.Close()
	}
	return nil
}

func (c *LocalContext) ConnectUpstream(addr string) (*sshutils.Upstream, error) {
	agent, err := c.clt.GetAgent()
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %v", err)
	}
	signers, err := agent.Signers()
	if err != nil {
		return nil, fmt.Errorf("no signers: %v", err)
	}
	return sshutils.DialUpstream(c.user, addr, signers)
}

type RequestHandler func(http.ResponseWriter, *http.Request, httprouter.Params, Context)

type AuthHandler interface {
	GetHost() string
	Auth(user, pass string, hotpToken string) (string, error)
	ValidateSession(user, sid string) (Context, error)
	SetSession(w http.ResponseWriter, user, sid string) error
	ClearSession(w http.ResponseWriter)
}

func NewLocalAuth(host string, servers []utils.NetAddr) (*LocalAuth, error) {
	m, err := ttlmap.NewMap(1024, ttlmap.CallOnExpire(CloseContext))
	if err != nil {
		return nil, err
	}
	return &LocalAuth{
		host:        host,
		sessions:    m,
		authServers: servers,
	}, nil
}

type LocalAuth struct {
	sessions    *ttlmap.TtlMap
	authServers []utils.NetAddr
	host        string
}

func (s *LocalAuth) GetHost() string {
	return s.host
}

func CloseContext(key string, val interface{}) {
	log.Infof("closing context %v", key)
	ctx := val.(Context)
	err := ctx.Close()
	if err != nil {
		log.Errorf("failed closing context: %v", err)
	}
}

func (s *LocalAuth) Auth(user, pass string, hotpToken string) (string, error) {
	method, err := auth.NewWebPasswordAuth(user, []byte(pass), hotpToken)
	if err != nil {
		return "", err
	}
	clt, err := auth.NewTunClient(s.authServers[0], user, method)
	if err != nil {
		return "", err
	}
	return clt.SignIn(user, []byte(pass))
}

func (s *LocalAuth) ValidateSession(user, sid string) (Context, error) {
	val, ok := s.sessions.Get(user + sid)
	if ok {
		return val.(Context), nil
	}

	method, err := auth.NewWebSessionAuth(user, []byte(sid))
	if err != nil {
		return nil, err
	}
	clt, err := auth.NewTunClient(s.authServers[0], user, method)
	if err != nil {
		log.Infof("failed to connect: %v", clt, err)
		return nil, err
	}
	if _, err := clt.GetWebSession(user, sid); err != nil {
		log.Infof("session not found: %v", err)
		return nil, err
	}
	log.Infof("session validated")

	c := &LocalContext{
		clt:  clt,
		user: user,
		sid:  sid,
	}
	if err := s.sessions.Set(user+sid, c, 600); err != nil {
		log.Infof("something is wrong: %v", err)
		return nil, err
	}
	return c, nil
}

func (s *LocalAuth) SetSession(w http.ResponseWriter, user, sid string) error {
	d, err := EncodeCookie(user, sid)
	if err != nil {
		return err
	}
	c := &http.Cookie{
		Domain: fmt.Sprintf(".%v", s.host),
		Name:   "session",
		Value:  d,
		Path:   "/",
	}
	http.SetCookie(w, c)
	return nil
}

func (s *LocalAuth) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Domain: fmt.Sprintf(".%v", s.host),
		Name:   "session",
		Value:  "",
		Path:   "/",
	})
}
