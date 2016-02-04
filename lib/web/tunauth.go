/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package web

import (
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/log"
)

type TunAuth struct {
	AuthHandler
	siteName string
	srv      reversetunnel.Server
}

func NewTunAuth(auth AuthHandler, srv reversetunnel.Server, siteName string) (*TunAuth, error) {
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
	site reversetunnel.RemoteSite
}

func (c *TunContext) ConnectUpstream(addr string, user string) (*sshutils.Upstream, error) {
	methods, err := c.GetAuthMethods()
	if err != nil {
		return nil, err
	}
	client, err := c.site.ConnectToServer(addr, user, methods)
	if err != nil {
		return nil, err
	}
	return sshutils.NewUpstream(client)
}

func (c *TunContext) GetClient() auth.ClientI {
	return c.site.GetClient()
}
