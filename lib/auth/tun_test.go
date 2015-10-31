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
package auth

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gokyle/hotp"

	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type TunSuite struct {
	bk   *encryptedbk.ReplicatedBackend
	scrt secret.SecretService

	srv    *httptest.Server
	tsrv   *TunServer
	a      *AuthServer
	signer ssh.Signer
	bl     *boltlog.BoltLog
	dir    string
	rec    recorder.Recorder
}

var _ = Suite(&TunSuite{})

func (s *TunSuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
	log.Initialize("console", "WARN")

}

func (s *TunSuite) TearDownTest(c *C) {
	s.srv.Close()
}

func (s *TunSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(s.dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	s.bl, err = boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)

	s.rec, err = boltrec.New(s.dir)
	c.Assert(err, IsNil)

	s.a = NewAuthServer(s.bk, authority.New(), s.scrt)
	s.srv = httptest.NewServer(
		NewAPIServer(s.a, s.bl, session.New(s.bk), s.rec))

	// set up host private key and certificate
	c.Assert(s.a.ResetHostCA(""), IsNil)
	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "localhost", "localhost", 0)
	c.Assert(err, IsNil)

	signer, err := sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)
	s.signer = signer
	u, err := url.Parse(s.srv.URL)
	c.Assert(err, IsNil)

	tsrv, err := NewTunServer(
		utils.NetAddr{Network: "tcp", Addr: "127.0.0.1:0"},
		[]ssh.Signer{signer},
		utils.NetAddr{Network: "tcp", Addr: u.Host}, s.a)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv
}

func (s *TunSuite) TestUnixServerClient(c *C) {
	d, err := ioutil.TempDir("", "teleport-test")
	c.Assert(err, IsNil)
	socketPath := filepath.Join(d, "unix.sock")

	l, err := net.Listen("unix", socketPath)
	c.Assert(err, IsNil)

	h := NewAPIServer(s.a, s.bl, session.New(s.bk), s.rec)
	srv := &httptest.Server{
		Listener: l,
		Config: &http.Server{
			Handler: h,
		},
	}
	srv.Start()
	defer srv.Close()

	u, err := url.Parse(s.srv.URL)
	c.Assert(err, IsNil)

	tsrv, err := NewTunServer(
		utils.NetAddr{Network: "tcp", Addr: "127.0.0.1:0"},
		[]ssh.Signer{s.signer},
		utils.NetAddr{Network: "tcp", Addr: u.Host}, s.a)

	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)
	s.tsrv = tsrv

	user := "test"
	pass := []byte("pwd123")

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "test")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: tsrv.Addr()},
		"test", authMethod)
	c.Assert(err, IsNil)

	err = clt.UpsertServer(
		services.Server{ID: "a.example.com", Addr: "hello"}, 0)
	c.Assert(err, IsNil)
}

func (s *TunSuite) TestSessions(c *C) {
	c.Assert(s.a.ResetUserCA(""), IsNil)

	user := "ws-test"
	pass := []byte("ws-abc123")

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "ws-test")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	hotpURL, _, err = clt.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err = hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "ws-test")
	otp.Increment()

	ws, err := clt.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, Not(Equals), "")

	out, err := clt.GetWebSession(user, ws)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	// Resume session via sesison id
	authMethod, err = NewWebSessionAuth(user, []byte(ws))
	c.Assert(err, IsNil)

	cltw, err := NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer cltw.Close()

	err = cltw.DeleteWebSession(user, ws)
	c.Assert(err, IsNil)

	_, err = clt.GetWebSession(user, ws)
	c.Assert(err, NotNil)
}

func (s *TunSuite) TestSessionsBadPassword(c *C) {
	c.Assert(s.a.ResetUserCA(""), IsNil)

	user := "system-test"
	pass := []byte("system-abc123")

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "system-test")
	otp.Increment()

	authMethod, err := NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	clt, err := NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: s.tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer clt.Close()

	ws, err := clt.SignIn(user, []byte("different-pass"))
	c.Assert(err, NotNil)
	c.Assert(ws, Equals, "")

	ws, err = clt.SignIn("not-exitsts", pass)
	c.Assert(err, NotNil)
	c.Assert(ws, Equals, "")

}
