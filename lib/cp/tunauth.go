package cp

import (
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tun"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
)

type TunAuth struct {
	AuthHandler
	siteName string
	srv      tun.Server
}

func NewTunAuth(auth AuthHandler, srv tun.Server, siteName string) (*TunAuth, error) {
	t := &TunAuth{srv: srv, siteName: siteName}
	t.AuthHandler = auth
	return t, nil
}

func (t *TunAuth) ValidateSession(user, sid string) (Context, error) {
	lctx, err := t.AuthHandler.ValidateSession(user, sid)
	if err != nil {
		return nil, err
	}
	site, err := t.srv.GetSite(t.siteName)
	if err != nil {
		log.Infof("failed to find site: %v %v", t.siteName, err)
		return nil, err
	}
	tctx := &TunContext{site: site}
	tctx.Context = lctx
	return tctx, nil
}

type TunContext struct {
	Context
	site tun.RemoteSite
}

func (c *TunContext) ConnectUpstream(addr string) (*sshutils.Upstream, error) {
	methods, err := c.GetAuthMethods()
	if err != nil {
		return nil, err
	}
	client, err := c.site.ConnectToServer(addr, c.GetUser(), methods)
	if err != nil {
		return nil, err
	}
	return sshutils.NewUpstream(client)
}

func (c *TunContext) GetClient() auth.ClientI {
	return c.site.GetClient()
}
