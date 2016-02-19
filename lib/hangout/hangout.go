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
	"net"
	"os"
	"os/user"
	"path"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/backend/etcdbk"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Hangout struct {
	auth             *auth.AuthServer
	signer           ssh.Signer
	tunClt           reversetunnel.Agent
	elog             events.Log
	rec              recorder.Recorder
	userPassword     string
	HangoutID        string
	nodePort         string
	authPort         string
	ClientAuthMethod ssh.AuthMethod
	HostKeyCallback  utils.HostKeyCallback
	client           *auth.TunClient
	HangoutInfo      utils.HangoutInfo
	Token            string
	sessions         session.SessionServer
}

func New(proxyTunnelAddress, nodeListeningAddress, authListeningAddress string,
	readOnly bool, authMethods []ssh.AuthMethod,
	hostKeyCallback utils.HostKeyCallback) (*Hangout, error) {

	//log.SetOutput(os.Stderr)
	//log.SetLevel(log.InfoLevel)

	cfg := service.Config{}
	service.ApplyDefaults(&cfg)
	subdir, err := auth.CryptoRandomHex(10)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.DataDir = HangoutDataDir + "/" + subdir
	cfg.Hostname = "localhost"

	cfg.Auth.HostAuthorityDomain = "localhost"
	cfg.Auth.KeysBackend.Type = "bolt"
	cfg.Auth.KeysBackend.Params = `{"path": "` + cfg.DataDir + `/teleport.auth.db"}`
	cfg.Auth.EventsBackend.Type = "bolt"
	cfg.Auth.EventsBackend.Params = `{"path": "` + cfg.DataDir + `/teleport.event.db"}`
	cfg.Auth.RecordsBackend.Type = "bolt"
	cfg.Auth.RecordsBackend.Params = `{"path": "` + cfg.DataDir + `/teleport.records.db"}`
	authAddress, err := utils.ParseAddr("tcp://" + authListeningAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.Auth.SSHAddr = *authAddress
	cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}

	nodeAddress, err := utils.ParseAddr("tcp://" + nodeListeningAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.SSH.Addr = *nodeAddress

	tunnelAddress, err := utils.ParseAddr("tcp://" + proxyTunnelAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.ReverseTunnel.DialAddr = *tunnelAddress

	_, err = os.Stat(cfg.DataDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(cfg.DataDir, os.ModeDir|0777)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	h := &Hangout{}

	h.HangoutID, err = auth.CryptoRandomHex(20)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := h.initAuth(cfg, readOnly); err != nil {
		return nil, trace.Wrap(err)
	}

	thisSrv := services.Server{
		ID:       cfg.Auth.SSHAddr.Addr,
		Addr:     cfg.Auth.SSHAddr.Addr,
		Hostname: h.HangoutID,
	}
	err = h.auth.UpsertServer(thisSrv, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := h.initSSHEndpoint(cfg); err != nil {
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

	h.ClientAuthMethod, err = Authorize(h.client)
	h.HostKeyCallback = nil

	h.HangoutInfo.AuthPort = h.authPort
	h.HangoutInfo.NodePort = h.nodePort
	h.HangoutInfo.HangoutID = h.HangoutID

	h.Token, err = utils.MarshalHangoutInfo(&h.HangoutInfo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// saving hangoutInfo using sessions just as storage
	err = h.sessions.UpsertSession(h.Token, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := h.initTunAgent(cfg, authMethods, hostKeyCallback); err != nil {
		return nil, trace.Wrap(err)
	}

	return h, nil
}

func (h *Hangout) createUser() error {
	var err error
	h.userPassword, err = auth.CryptoRandomHex(20)
	if err != nil {
		return trace.Wrap(err)
	}

	u, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}
	osUser := u.Username
	h.HangoutInfo.OSUser = osUser

	_, _, err = h.auth.UpsertPassword(HangoutUser, []byte(h.userPassword))
	if err != nil {
		return trace.Wrap(err)
	}

	if err = h.auth.UpsertUser(services.User{Name: HangoutUser, AllowedLogins: []string{osUser}}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func Authorize(auth auth.ClientI) (ssh.AuthMethod, error) {

	priv, pub, err := authority.New().GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := auth.GenerateUserCert(pub, HangoutUser, 24*time.Hour)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pk, err := ssh.ParseRawPrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
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
		return nil, trace.Wrap(err)
	}

	return ssh.PublicKeysCallback(ag.Signers), nil
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
	acfg := auth.InitConfig{
		Backend:       b,
		Authority:     authority.New(),
		DomainName:    cfg.Hostname,
		AuthDomain:    cfg.Auth.HostAuthorityDomain,
		DataDir:       cfg.DataDir,
		SecretKey:     cfg.Auth.SecretKey,
		AllowedTokens: cfg.Auth.AllowedTokens,
	}
	asrv, signer, err := auth.Init(acfg)
	if err != nil {
		return trace.Wrap(err)
	}
	h.signer = signer
	h.auth = asrv
	h.sessions = session.New(b)
	apisrv := auth.NewAPIWithRoles(asrv, h.elog, h.sessions, h.rec,
		auth.NewHangoutPermissions(), auth.HangoutRoles,
	)
	go apisrv.Serve()

	limiter, err := limiter.NewLimiter(cfg.Auth.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("[AUTH] server SSH endpoint is starting")
	tsrv, err := auth.NewTunServer(
		cfg.Auth.SSHAddr, []ssh.Signer{h.signer},
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

	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		cfg.Hostname,
		[]ssh.AuthMethod{ssh.PublicKeys(h.signer)})
	if err != nil {
		return trace.Wrap(err)
	}

	h.client = client

	return nil
}

func (h *Hangout) initTunAgent(cfg service.Config, authMethods []ssh.AuthMethod, hostKeyCallback utils.HostKeyCallback) error {

	elog := &service.FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.StandardLogger().Writer()),
			h.client,
		}}

	a, err := reversetunnel.NewHangoutAgent(
		cfg.ReverseTunnel.DialAddr,
		h.HangoutID,
		authMethods,
		hostKeyCallback,
		h.client,
		reversetunnel.SetEventLogger(elog))
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("[REVERSE TUNNEL] teleport tunnel agent starting")
	if err := a.Start(); err != nil {
		log.Fatalf("failed to start: %v", err)
		return trace.Wrap(err)
	}

	go func() {
		if err := a.Wait(); err != nil {
			log.Fatalf("Can't connect to the remote proxy: %v\n", err)
		}
	}()
	return nil
}

func (h *Hangout) initSSHEndpoint(cfg service.Config) error {
	elog := &service.FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.StandardLogger().Writer()),
			h.client,
		},
	}

	limiter, err := limiter.NewLimiter(cfg.SSH.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	s, err := srv.New(cfg.SSH.Addr,
		h.HangoutID,
		[]ssh.Signer{h.signer},
		h.client,
		limiter,
		cfg.DataDir,
		srv.SetShell(cfg.SSH.Shell),
		srv.SetEventLogger(elog),
		srv.SetSessionServer(h.client),
		srv.SetRecorder(h.client),
		srv.SetLabels(cfg.SSH.Labels, cfg.SSH.CmdLabels),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("[SSH] server is starting on %v", cfg.SSH.Addr)
	go func() {
		if err := s.Start(); err != nil {
			log.Errorf(err.Error())
		}
		s.Wait()
	}()
	return nil
}

func initBackend(dataDir, domainName string, peers service.NetAddrSlice, cfg service.AuthConfig) (*encryptedbk.ReplicatedBackend, error) {
	var bk backend.Backend
	var err error

	switch cfg.KeysBackend.Type {
	case "etcd":
		bk, err = etcdbk.FromJSON(cfg.KeysBackend.Params)
	case "bolt":
		bk, err = boltbk.FromJSON(cfg.KeysBackend.Params)
	default:
		return nil, trace.Errorf("unsupported backend type: %v", cfg.KeysBackend.Type)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyStorage := path.Join(dataDir, "backend_keys")
	encryptionKeys := []encryptor.Key{}
	for _, strKey := range cfg.KeysBackend.EncryptionKeys {
		encKey, err := encryptedbk.KeyFromString(strKey)
		if err != nil {
			return nil, err
		}
		encryptionKeys = append(encryptionKeys, encKey)
	}

	encryptedBk, err := encryptedbk.NewReplicatedBackend(bk,
		keyStorage, encryptionKeys, encryptor.GenerateGPGKey)

	if err != nil {
		log.Errorf(err.Error())
		log.Infof("Initializing backend as follower node")
		myKey, err := encryptor.GenerateGPGKey(domainName + " key")
		if err != nil {
			return nil, err
		}
		masterKey, err := auth.RegisterNewAuth(
			domainName, cfg.Token, myKey.Public(), peers)
		if err != nil {
			return nil, err
		}
		log.Infof(" ", myKey, masterKey)
		encryptedBk, err = encryptedbk.NewReplicatedBackend(bk,
			keyStorage, []encryptor.Key{myKey, masterKey},
			encryptor.GenerateGPGKey)
		if err != nil {
			return nil, err
		}
	}
	return encryptedBk, nil
}

func initEventBackend(btype string, params string) (events.Log, error) {
	switch btype {
	case "bolt":
		return boltlog.FromJSON(params)
	}
	return nil, trace.Errorf("unsupported backend type: %v", btype)
}

func initRecordBackend(btype string, params string) (recorder.Recorder, error) {
	switch btype {
	case "bolt":
		return boltrec.FromJSON(params)
	}
	return nil, trace.Errorf("unsupported backend type: %v", btype)
}

const HangoutUser = "hangoutUser"
const HangoutDataDir = "/tmp/teleport_hangouts"
const DefaultNodeAddress = "localhost:3031"
const DefaultAuthAddress = "localhost:3032"
