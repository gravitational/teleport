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
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/roundtrip"
	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

func TestWeb(t *testing.T) { TestingT(t) }

type WebSuite struct {
	node        *srv.Server
	srvAddress  string
	srvHostPort string
	bk          *encryptedbk.ReplicatedBackend
	roleAuth    *auth.AuthWithRoles
	dir         string
	user        string
	domainName  string
	signer      ssh.Signer
	tunServer   *auth.TunServer
	webServer   *httptest.Server
}

var _ = Suite(&WebSuite{})

func (s *WebSuite) SetUpSuite(c *C) {
	utils.InitLoggerDebug()
}

func (s *WebSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	u, err := user.Current()
	c.Assert(err, IsNil)
	s.user = u.Username

	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(s.dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	s.domainName = "localhost"
	authServer := auth.NewAuthServer(s.bk, authority.New(), s.domainName)

	eventsLog, err := boltlog.New(filepath.Join(s.dir, "boltlog"))
	c.Assert(err, IsNil)

	c.Assert(authServer.UpsertCertAuthority(
		*services.NewTestCA(services.UserCA, s.domainName), backend.Forever), IsNil)
	c.Assert(authServer.UpsertCertAuthority(
		*services.NewTestCA(services.HostCA, s.domainName), backend.Forever), IsNil)

	recorder, err := boltrec.New(s.dir)
	c.Assert(err, IsNil)

	s.roleAuth = auth.NewAuthWithRoles(authServer,
		auth.NewStandardPermissions(),
		eventsLog,
		sess.New(baseBk),
		teleport.RoleAdmin,
		recorder)

	// set up host private key and certificate
	hpriv, hpub, err := authServer.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := authServer.GenerateHostCert(
		hpub, s.domainName, s.domainName, teleport.RoleAdmin, 0)
	c.Assert(err, IsNil)

	// set up user CA and set up a user that has access to the server
	s.signer, err = sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)

	limiter, err := limiter.NewLimiter(
		limiter.LimiterConfig{
			MaxConnections: 100,
			Rates: []limiter.Rate{
				limiter.Rate{
					Period:  1 * time.Second,
					Average: 100,
					Burst:   400,
				},
				limiter.Rate{
					Period:  40 * time.Millisecond,
					Average: 1000,
					Burst:   4000,
				},
			},
		},
	)
	c.Assert(err, IsNil)

	// start node
	nodePort, err := utils.GetFreeTCPPort()
	c.Assert(err, IsNil)

	s.srvAddress = fmt.Sprintf("127.0.0.1:%v", nodePort)
	node, err := srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.srvAddress},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		limiter,
		s.dir,
		srv.SetShell("/bin/sh"),
	)
	c.Assert(err, IsNil)
	s.node = node

	c.Assert(s.node.Start(), IsNil)

	revTunServer, err := reversetunnel.NewServer(
		utils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        fmt.Sprintf("%v:0", s.domainName),
		},
		[]ssh.Signer{s.signer},
		s.roleAuth, limiter,
		reversetunnel.ServerTimeout(200*time.Millisecond),
		reversetunnel.DirectSite(s.domainName, s.roleAuth),
	)
	c.Assert(err, IsNil)

	apiPort, err := utils.GetFreeTCPPort()
	c.Assert(err, IsNil)

	apiServer := auth.NewAPIWithRoles(authServer, eventsLog, sess.New(s.bk), recorder,
		auth.NewAllowAllPermissions(),
		auth.StandardRoles,
	)
	go apiServer.Serve()

	tunAddr := utils.NetAddr{
		AddrNetwork: "tcp", Addr: fmt.Sprintf("127.0.0.1:%v", apiPort),
	}

	s.tunServer, err = auth.NewTunServer(
		tunAddr,
		[]ssh.Signer{s.signer},
		apiServer, authServer, limiter)
	c.Assert(err, IsNil)
	c.Assert(s.tunServer.Start(), IsNil)

	// start handler
	handler, err := NewMultiSiteHandler(MultiSiteConfig{
		InsecureHTTPMode: true,
		Tun:              revTunServer,
		AssetsDir:        "assets/web",
		AuthAddr:         tunAddr,
		DomainName:       s.domainName,
	})

	s.webServer = httptest.NewServer(handler)
}

func (s *WebSuite) client() *roundtrip.Client {
	clt, err := roundtrip.NewClient("http://"+s.webServer.Listener.Addr().String(), "v1")
	if err != nil {
		panic(err)
	}
	return clt
}

func (s *WebSuite) TearDownTest(c *C) {
	c.Assert(s.node.Close(), IsNil)
	c.Assert(s.tunServer.Close(), IsNil)
	s.webServer.Close()
}

func (s *WebSuite) TestNewUser(c *C) {
	token, err := s.roleAuth.CreateSignupToken("bob", []string{s.user})
	c.Assert(err, IsNil)

	clt := s.client()
	re, err := clt.Get(clt.Endpoint("webapi", "users", "invites", token), url.Values{})
	c.Assert(err, IsNil)

	var out *renderUserInviteResponse
	c.Assert(json.Unmarshal(re.Bytes(), &out), IsNil)
	c.Assert(out.User, Equals, "bob")
	c.Assert(out.InviteToken, Equals, token)
}
