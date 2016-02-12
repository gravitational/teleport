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
package hangout

import (
	//"bytes"
	//"fmt"
	//"io"
	"os/user"
	//"path/filepath"
	//"strings"
	//"time"
	"net"
	"os"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/native"
	//"github.com/gravitational/teleport/lib/backend/boltbk"
	//"github.com/gravitational/teleport/lib/backend/encryptedbk"
	//"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events"
	//"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/recorder"
	//"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	//"github.com/gravitational/teleport/lib/services"
	//sess "github.com/gravitational/teleport/lib/session"
	//"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/session"
	"github.com/gravitational/trace"
	//"github.com/gokyle/hotp"
	//"github.com/mailgun/lemma/secret"
	"golang.org/x/crypto/ssh"
	//"golang.org/x/crypto/ssh/agent"
)

type Hangout struct {
	auth             auth.AuthServer
	tunClt           reversetunnel.Agent
	elog             events.Log
	rec              recorder.Recorder
	userPassword     string
	hangoutID        string
	nodePort         string
	authPort         string
	ClientAuthMethod ssh.AuthMethod
	HostKeyCallback  utils.HostKeyCallback
}

func New(proxyTunnelAddress, nodeListeningAddress, authListeningAddress string,
	readOnly bool, authMethods []ssh.AuthMethod,
	hostKeyCallback utils.HostKeyCallback) (*Hangout, error) {

	cfg := service.Config{}
	service.SetDefaults(&cfg)
	cfg.DataDir = "/tmp/teleporthangout"
	cfg.Hostname = "localhost"

	cfg.Auth.HostAuthorityDomain = "localhost"
	cfg.Auth.KeysBackend.Type = "bolt"
	cfg.Auth.KeysBackend.Params = `{"path": "` + DataDir + `/teleport.auth.db"}`
	cfg.Auth.EventsBackend.Type = "bolt"
	cfg.Auth.EventsBackend.Params = `{"path": "` + DataDir + `/teleport.event.db"}`
	cfg.Auth.RecordsBackend.Type = "bolt"
	cfg.Auth.RecordsBackend.Params = `{"path": "` + DataDir + `/teleport.records.db"}`
	authAddress, err := utils.ParseAddr(authListeningAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.Auth.SSHAddr = authAddress
	cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}

	nodeAddress, err := utils.ParseAddr(nodeListeningAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.SSH.Addr = nodeAddress

	tunnelAddress, err := utils.ParseAddr(proxyTunnelAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.ReverseTunnel.DialAddr = tunnelAddress

	_, err := os.Stat(cfg.DataDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(cfg.DataDir, os.ModeDir|0777)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	h := &Hangout{}
	if err := h.initAuth(cfg, readOnly); err != nil {
		return nil, trace.Wrap(err)
	}

	var err error

	h.hangoutID, err = h.auth.CreateToken()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := h.initSSHEndpoint(cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := h.initTunAgent(cfg, authMethods, hostKeyCallback); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := h.createUser(); err != nil {
		return nil, trace.Wrap(err)
	}

	_, h.authPort, err = net.SplitHostPort(authListeningAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, h.nodePort, err = net.SplitHostPort(nodeListeningAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h.ClientAuthMethod, h.HostKeyCallback, err = Authorize(auth, h.userPassword)
}

func (h *Hangout) createUser() error {
	var err error
	h.userPassword, err = h.auth.GenerateToken(HangoutUser, services.TokenRoleHangout, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	u, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}
	osUser := u.Username

	_, _, err := s.a.UpsertPassword(HangoutUser, h.userPassword)
	if err != nil {
		return trace.Wrap(err)
	}

	err = h.auth.UpsertUserMapping("local", HangoutUser, osUser, 0)
}

func Authorize(auth auth.Client, userPassword string) (ssh.AuthMethod, utils.HostKeyCallback, error) {

	priv, pub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cert, err := auth.GenerateUserCert(pub, "id_"+HangoutUser, HangoutUser, 24*time.Hour)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(login.Cert)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	pk, err := ssh.ParseRawPrivateKey(priv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	addedKey := agent.AddedKey{
		PrivateKey:       pk,
		Certificate:      pcert.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}
	ag := agent.NewKeyring()
	if err := ag.Add(addedKey); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	/*hostSigners, err := auth.GetTrustedCertificates(services.HostCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}*/

	return ssh.PublicKeysCallback(ag.Signers), nil, nil

}

func (h *Hangout) initAuth(cfg service.Config, readOnlyHangout bool) error {
	if cfg.Auth.HostAuthorityDomain == "" {
		return trace.Errorf(
			"please provide host certificate authority domain, e.g. example.com")
	}

	b, err := initBackend(cfg.DataDir, cfg.Hostname, cfg.AuthServers, cfg.Auth)
	if err != nil {
		return trace.Wrap(err)
	}

	h.elog, err = initEventBackend(
		cfg.Auth.EventsBackend.Type, cfg.Auth.EventsBackend.Params)
	if err != nil {
		return trace.Wrap(err)
	}

	h.rec, err = initRecordBackend(
		cfg.Auth.RecordsBackend.Type, cfg.Auth.RecordsBackend.Params)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedAuthorities, err := cfg.Auth.TrustedAuthorities.Authorities()
	if err != nil {
		return trace.Wrap(err)
	}
	acfg := auth.InitConfig{
		Backend:            b,
		Authority:          authority.New(),
		DomainName:         cfg.Hostname,
		AuthDomain:         cfg.Auth.HostAuthorityDomain,
		DataDir:            cfg.DataDir,
		SecretKey:          cfg.Auth.SecretKey,
		AllowedTokens:      cfg.Auth.AllowedTokens,
		TrustedAuthorities: trustedAuthorities,
	}
	asrv, signer, err := auth.Init(acfg)
	if err != nil {
		return trace.Wrap(err)
	}
	apisrv := auth.NewAPIWithRoles(asrv, elog, session.New(b), rec,
		auth.NewStandardPermissions(), auth.HangoutRoles,
	)
	go apisrv.Serve()

	limiter, err := limiter.NewLimiter(cfg.Auth.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("[AUTH] server SSH endpoint is starting")
	tsrv, err := auth.NewTunServer(
		cfg.Auth.SSHAddr, []ssh.Signer{signer},
		apisrv,
		asrv,
		limiter,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		if err := tsrv.Start(); err != nil {
			log.Errorf(err.Error())
		}
	}()

	thisSrv := services.Server{
		ID:       cfg.Auth.SSHAddr.Addr(),
		Addr:     cfg.Auth.SSHAddr.Addr(),
		Hostname: h.HangoutID,
	}
	err := asrv.UpsertServer(thisSrv, 0)
	return nil
}

func (h *Hangout) initTunAgent(cfg Config, authMethods []ssh.AuthMethod, hostKeyCallback utils.HostKeyCallback) error {
	signer, err := auth.ReadKeys(cfg.Hostname, cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		cfg.Hostname,
		[]ssh.AuthMethod{ssh.PublicKeys(signer)})
	if err != nil {
		return trace.Wrap(err)
	}

	elog := &service.FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.StandardLogger().Writer()),
			client,
		}}

	a, err := reversetunnel.NewHangoutAgent(
		cfg.ReverseTunnel.DialAddr,
		h.hangoutID,
		authMethods,
		hostKeyCallback,
		client,
		reversetunnel.SetEventLogger(elog))
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("[REVERSE TUNNEL] teleport tunnel agent starting")
	go func() {
		if err := a.Start(); err != nil {
			log.Fatalf("failed to start: %v", err)
		}
		if err := a.Wait(); err != nil {
			log.Fatalf("failed to start: %v", err)
		}
	}()
	return nil
}

func (h *Hangout) initSSHEndpoint(cfg Config) error {
	signer, err := auth.ReadKeys(cfg.Hostname, cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		cfg.Hostname,
		[]ssh.AuthMethod{ssh.PublicKeys(signer)})
	if err != nil {
		return trace.Wrap(err)
	}

	elog := &service.FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.StandardLogger().Writer()),
			client,
		},
	}

	limiter, err := limiter.NewLimiter(cfg.SSH.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	s, err := srv.New(cfg.SSH.Addr,
		cfg.Hostname,
		[]ssh.Signer{signer},
		client,
		limiter,
		cfg.DataDir,
		srv.SetShell(cfg.SSH.Shell),
		srv.SetEventLogger(elog),
		srv.SetSessionServer(client),
		srv.SetRecorder(client),
		srv.SetLabels(cfg.SSH.Labels, cfg.SSH.CmdLabels),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("[SSH] server is starting on %v", cfg.SSH.Addr)
	go func() {
		if err := s.Start(); err != nil {
			log.Errorf(err)
		}
		if err := s.Wait(); err != nil {
			log.Errorf(err)
		}
	}()
	return nil
}

func (h *Hangout) GetJoinCommand() string {
	nodeAddress := h.hangoutID + ":" + h.nodePort
	authAddress := h.hangoutID + ":" + h.authPort
}

const HangoutUser = "hangoutUser"
