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

// Package service implements teleport running service, takes care
// of initialization, cleanup and shutdown procedures
package service

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/etcdbk"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

type RoleConfig struct {
	DataDir     string
	HostUUID    string
	HostName    string
	AuthServers []utils.NetAddr
	Auth        AuthConfig
	Console     io.Writer
}

// connector has all resources process needs to connect
// to other parts of the cluster: client and identity
type connector struct {
	identity *auth.Identity
	client   *auth.TunClient
}

// TeleportProcess structure holds the state of the Teleport daemon, controlling
// execution and configuration of the teleport services: ssh, auth and proxy.
type TeleportProcess struct {
	sync.Mutex
	Supervisor
	Config *Config
	// localAuth has local auth server listed in case if this process
	// has started with auth server role enabled
	localAuth *auth.AuthServer
}

// loginIntoAuthService attempts to login into the auth servers specified in the
// configuration. Returns 'true' if successful
func (process *TeleportProcess) connectToAuthService(role teleport.Role) (*connector, error) {
	identity, err := auth.ReadIdentity(
		process.Config.DataDir, auth.IdentityID{HostUUID: process.Config.HostUUID, Role: role})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authUser := identity.Cert.ValidPrincipals[0]
	authClient, err := auth.NewTunClient(
		process.Config.AuthServers[0],
		authUser,
		[]ssh.AuthMethod{ssh.PublicKeys(identity.KeySigner)})
	// success?
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// try calling a test method via auth api:
	_, err = authClient.GetLocalDomain()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// success ? we're logged in!
	log.Infof("%s connected to the cluster", authUser)
	return &connector{client: authClient, identity: identity}, nil
}

// NewTeleport takes the daemon configuration, instantiates all required services
// and starts them under a supervisor, returning the supervisor object
func NewTeleport(cfg *Config) (Supervisor, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	// create the data directory if it's missing
	_, err := os.Stat(cfg.DataDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(cfg.DataDir, os.ModeDir|0777)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// read or generate a host UUID for this node
	cfg.HostUUID, err = utils.ReadOrMakeHostUUID(cfg.DataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if user started auth and another service (without providing the auth address for
	// that service, the address of the in-process auth will be used
	if cfg.Auth.Enabled && len(cfg.AuthServers) == 0 {
		cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}
	}

	// if user did not provide auth domain name, use this host UUID
	if cfg.Auth.Enabled && cfg.Auth.DomainName == "" {
		cfg.Auth.DomainName = cfg.HostUUID
	}

	// try to login into the auth service:

	// if there are no certificates, use self signed
	process := &TeleportProcess{
		Supervisor: NewSupervisor(),
		Config:     cfg,
	}

	serviceStarted := false

	if cfg.Auth.Enabled {
		if err := process.initAuthService(); err != nil {
			return nil, trace.Wrap(err)
		}
		serviceStarted = true
	}

	if cfg.SSH.Enabled {
		if err := process.initSSH(); err != nil {
			return nil, err
		}
		serviceStarted = true
	}

	if cfg.ReverseTunnel.Enabled {
		if err := process.initReverseTunnel(); err != nil {
			return nil, err
		}
		serviceStarted = true
	}

	if cfg.Proxy.Enabled {
		if err := process.initProxy(); err != nil {
			return nil, err
		}
		serviceStarted = true
	}

	if !serviceStarted {
		return nil, trace.Errorf("all services failed to start")
	}

	return process, nil
}

func (process *TeleportProcess) setLocalAuth(a *auth.AuthServer) {
	process.Lock()
	defer process.Unlock()
	process.localAuth = a
}

func (process *TeleportProcess) getLocalAuth() *auth.AuthServer {
	process.Lock()
	defer process.Unlock()
	return process.localAuth
}

// initAuthService can be called to initialize auth server service
func (process *TeleportProcess) initAuthService() error {
	cfg := process.Config
	// Initialize the storage back-ends for keys, events and records
	b, err := process.initAuthStorage()
	if err != nil {
		return trace.Wrap(err)
	}
	elog, err := initEventStorage(
		cfg.Auth.EventsBackend.Type, cfg.Auth.EventsBackend.Params)
	if err != nil {
		return trace.Wrap(err)
	}
	rec, err := initRecordStorage(
		cfg.Auth.RecordsBackend.Type, cfg.Auth.RecordsBackend.Params)
	if err != nil {
		return trace.Wrap(err)
	}

	acfg := auth.InitConfig{
		Backend:         b,
		Authority:       authority.New(),
		DomainName:      cfg.Auth.DomainName,
		AuthServiceName: cfg.Hostname,
		DataDir:         cfg.DataDir,
		SecretKey:       cfg.Auth.SecretKey,
		AllowedTokens:   cfg.Auth.AllowedTokens,
		HostUUID:        cfg.HostUUID,
	}
	authServer, identity, err := auth.Init(acfg)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionService, err := session.New(b)
	if err != nil {
		return trace.Wrap(err)
	}
	// set local auth to use from the same process (to simplify setup
	// if there are some other roles started in the same process)
	process.setLocalAuth(authServer)

	apiServer := auth.NewAPIWithRoles(auth.APIConfig{
		AuthServer:        authServer,
		EventLog:          elog,
		SessionService:    sessionService,
		Recorder:          rec,
		PermissionChecker: auth.NewStandardPermissions(),
		Roles:             auth.StandardRoles,
	})
	process.RegisterFunc(func() error {
		apiServer.Serve()
		return nil
	})

	limiter, err := limiter.NewLimiter(cfg.Auth.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	// Register an SSH endpoint which is used to create an SSH tunnel to send HTTP
	// requests to the Auth API
	process.RegisterFunc(func() error {
		utils.Consolef(cfg.Console, "[AUTH]  Auth service is starting on %v", cfg.Auth.SSHAddr.Addr)
		tsrv, err := auth.NewTunnel(
			cfg.Auth.SSHAddr, []ssh.Signer{identity.KeySigner},
			apiServer,
			authServer,
			auth.SetLimiter(limiter),
		)
		if err != nil {
			utils.Consolef(cfg.Console, "[PROXY] Error: %v", err)
			return trace.Wrap(err)
		}
		if err := tsrv.Start(); err != nil {
			utils.Consolef(cfg.Console, "[PROXY] Error: %v", err)
			return trace.Wrap(err)
		}
		return nil
	})

	// Heart beat auth server presence, this is not the best place for this
	// logic, consolidate it into auth package later
	process.RegisterFunc(func() error {
		authClient, err := auth.NewTunClient(
			cfg.Auth.SSHAddr,
			identity.Cert.ValidPrincipals[0],
			[]ssh.AuthMethod{ssh.PublicKeys(identity.KeySigner)})
		// success?
		if err != nil {
			return trace.Wrap(err)
		}
		srv := services.Server{
			ID:       process.Config.HostUUID,
			Addr:     cfg.Auth.SSHAddr.Addr,
			Hostname: process.Config.Hostname,
		}
		if process.Config.AdvertiseIP != nil {
			_, port, err := net.SplitHostPort(srv.Addr)
			if err != nil {
				return trace.Wrap(err)
			}
			srv.Addr = fmt.Sprintf("%v:%v", process.Config.AdvertiseIP.String(), port)
		}
		for {
			err := authClient.UpsertAuthServer(srv, defaults.ServerHeartbeatTTL)
			if err != nil {
				log.Warningf("failed to announce presence: %v", err)
			}
			sleepTime := defaults.ServerHeartbeatTTL/2 + utils.RandomDuration(defaults.ServerHeartbeatTTL/10)
			log.Infof("[AUTH] will ping auth service in %v", sleepTime)
			time.Sleep(sleepTime)
		}
	})
	return nil
}

func (process *TeleportProcess) initSSH() error {
	return process.RegisterWithAuthServer(
		process.Config.SSH.Token, teleport.RoleNode,
		process.initSSHEndpoint)
}

func (process *TeleportProcess) initSSHEndpoint(conn *connector) error {
	cfg := process.Config

	limiter, err := limiter.NewLimiter(cfg.SSH.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	s, err := srv.New(cfg.SSH.Addr,
		cfg.Hostname,
		[]ssh.Signer{conn.identity.KeySigner},
		conn.client,
		cfg.DataDir,
		cfg.AdvertiseIP,
		srv.SetLimiter(limiter),
		srv.SetShell(cfg.SSH.Shell),
		srv.SetEventLogger(conn.client),
		srv.SetSessionServer(conn.client),
		srv.SetRecorder(conn.client),
		srv.SetLabels(cfg.SSH.Labels, cfg.SSH.CmdLabels),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	process.RegisterFunc(func() error {
		utils.Consolef(cfg.Console, "[SSH]   Service is starting on %v", cfg.SSH.Addr.Addr)
		if err := s.Start(); err != nil {
			utils.Consolef(cfg.Console, "[SSH]   Error: %v", err)
			return trace.Wrap(err)
		}
		s.Wait()
		return nil
	})
	return nil
}

// RegisterWithAuthServer uses one time provisioning token obtained earlier
// from the server to get a pair of SSH keys signed by Auth server host
// certificate authority
func (process *TeleportProcess) RegisterWithAuthServer(token string, role teleport.Role, callback func(conn *connector) error) error {
	cfg := process.Config
	identityID := auth.IdentityID{Role: role, HostUUID: cfg.HostUUID}

	// this means the server has not been initialized yet, we are starting
	// the registering client that attempts to connect to the auth server
	// and provision the keys
	process.RegisterFunc(func() error {
		for {
			conn, err := process.connectToAuthService(role)
			if err == nil {
				return callback(conn)
			}
			if teleport.IsConnectionProblem(err) {
				log.Errorf("[%v] failed connect to auth serverr: %v", role, err)
				time.Sleep(time.Second)
				continue
			}
			if !teleport.IsNotFound(err) {
				return trace.Wrap(err)
			}
			//  we haven't connected yet, so we expect the token to exist
			if process.getLocalAuth() != nil {
				// Auth service is on the same host, no need to go though the invitation
				// procedure
				log.Infof("this server has local Auth server started, using it to add role to the cluster")
				err = auth.LocalRegister(cfg.DataDir, identityID, process.getLocalAuth())
			} else {
				// Auth server is remote, so we need a provisioning token
				if token == "" {
					return trace.Wrap(teleport.BadParameter(role.String(), "role has no identity and no provisioning token"))
				}
				log.Infof("%v joining the cluster with a token %v", role, token)
				err = auth.Register(cfg.DataDir, token, identityID, cfg.AuthServers)
			}
			if err != nil {
				log.Errorf("[%v] failed to join the cluster: %v", role, err)
				time.Sleep(time.Second)
			} else {
				utils.Consolef(os.Stdout, "[%v] Successfully registered with the cluster", role)
				continue
			}
		}
	})
	return nil
}

func (process *TeleportProcess) initReverseTunnel() error {
	return process.RegisterWithAuthServer(
		process.Config.Proxy.Token,
		teleport.RoleNode,
		process.initTunAgent)
}

func (process *TeleportProcess) initTunAgent(conn *connector) error {
	cfg := process.Config

	a, err := reversetunnel.NewAgent(
		cfg.ReverseTunnel.DialAddr,
		cfg.Hostname,
		[]ssh.Signer{conn.identity.KeySigner},
		conn.client,
		reversetunnel.SetEventLogger(conn.client))
	if err != nil {
		return trace.Wrap(err)
	}

	process.RegisterFunc(func() error {
		log.Infof("[REVERSE TUNNEL] teleport tunnel agent starting")
		if err := a.Start(); err != nil {
			log.Fatalf("failed to start: %v", err)
			return trace.Wrap(err)
		}
		a.Wait()
		return nil
	})
	return nil
}

// initProxy gets called if teleport runs with 'proxy' role enabled.
// this means it will do two things:
//    1. serve a web UI
//    2. proxy SSH connections to nodes running with 'node' role
func (process *TeleportProcess) initProxy() (err error) {
	// if no TLS key was provided for the web UI, generate a self signed cert
	if process.Config.Proxy.TLSKey == "" {
		err = initSelfSignedHTTPSCert(process.Config)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return process.RegisterWithAuthServer(
		process.Config.Proxy.Token, teleport.RoleProxy,
		process.initProxyEndpoint)
}

func (process *TeleportProcess) initProxyEndpoint(conn *connector) error {
	cfg := process.Config
	proxyLimiter, err := limiter.NewLimiter(cfg.Proxy.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	reverseTunnelLimiter, err := limiter.NewLimiter(cfg.ReverseTunnel.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	tsrv, err := reversetunnel.NewServer(
		cfg.Proxy.ReverseTunnelListenAddr,
		[]ssh.Signer{conn.identity.KeySigner},
		conn.client,
		reversetunnel.SetLimiter(reverseTunnelLimiter),
		reversetunnel.DirectSite(conn.identity.Cert.Extensions[utils.CertExtensionAuthority], conn.client),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	SSHProxy, err := srv.New(cfg.Proxy.SSHAddr,
		cfg.Hostname,
		[]ssh.Signer{conn.identity.KeySigner},
		conn.client,
		cfg.DataDir,
		nil,
		srv.SetLimiter(proxyLimiter),
		srv.SetProxyMode(tsrv),
		srv.SetSessionServer(conn.client),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// register SSH reverse tunnel server that accepts connections
	// from remote teleport nodes
	process.RegisterFunc(func() error {
		utils.Consolef(cfg.Console, "[PROXY] Reverse tunnel service is starting on %v", cfg.Proxy.ReverseTunnelListenAddr.Addr)
		if err := tsrv.Start(); err != nil {
			utils.Consolef(cfg.Console, "[PROXY] Error: %v", err)
			return trace.Wrap(err)
		}
		tsrv.Wait()
		return nil
	})

	// Register web proxy server
	process.RegisterFunc(func() error {
		utils.Consolef(cfg.Console, "[PROXY] Web proxy service is starting on %v", cfg.Proxy.WebAddr.Addr)
		webHandler, err := web.NewHandler(
			web.Config{
				Proxy:       tsrv,
				AssetsDir:   cfg.Proxy.AssetsDir,
				AuthServers: cfg.AuthServers[0],
				DomainName:  cfg.Hostname})
		if err != nil {
			log.Errorf("failed to launch web server: %v", err)
			return err
		}

		proxyLimiter.WrapHandle(webHandler)

		log.Infof("[PROXY] init TLS listeners")
		err = utils.ListenAndServeTLS(
			cfg.Proxy.WebAddr.Addr,
			proxyLimiter,
			cfg.Proxy.TLSCert,
			cfg.Proxy.TLSKey)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	// Register ssh proxy server
	process.RegisterFunc(func() error {
		utils.Consolef(cfg.Console, "[PROXY] SSH proxy service is starting on %v", cfg.Proxy.SSHAddr.Addr)
		if err := SSHProxy.Start(); err != nil {
			utils.Consolef(cfg.Console, "[PROXY] Error: %v", err)
			return trace.Wrap(err)
		}
		return nil
	})

	return nil
}

// initAuthStorage initializes the storage backend for the auth. service
func (process *TeleportProcess) initAuthStorage() (backend.Backend, error) {
	cfg := &process.Config.Auth
	var bk backend.Backend
	var err error

	switch cfg.KeysBackend.Type {
	case teleport.ETCDBackendType:
		bk, err = etcdbk.FromJSON(cfg.KeysBackend.Params)
	case teleport.BoltBackendType:
		bk, err = boltbk.FromJSON(cfg.KeysBackend.Params)
	default:
		return nil, trace.Errorf("unsupported backend type: %v", cfg.KeysBackend.Type)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return bk, nil
}

func initEventStorage(btype string, params string) (events.Log, error) {
	switch btype {
	case "bolt":
		return boltlog.FromJSON(params)
	}
	return nil, trace.Errorf("unsupported backend type: %v", btype)
}

func initRecordStorage(btype string, params string) (recorder.Recorder, error) {
	switch btype {
	case "bolt":
		return boltrec.FromJSON(params)
	}
	return nil, trace.Errorf("unsupported backend type: %v", btype)
}

func validateConfig(cfg *Config) error {
	if !cfg.Auth.Enabled && !cfg.SSH.Enabled && !cfg.ReverseTunnel.Enabled {
		return trace.Wrap(
			teleport.BadParameter(
				"config", "supply at least one of Auth, SSH or ReverseTunnel or Proxy roles"))
	}

	if cfg.DataDir == "" {
		return trace.Wrap(teleport.BadParameter("config", "please supply data directory"))
	}

	if cfg.Console == nil {
		cfg.Console = ioutil.Discard
	}

	if (cfg.Proxy.TLSKey == "" && cfg.Proxy.TLSCert != "") || (cfg.Proxy.TLSKey != "" && cfg.Proxy.TLSCert == "") {
		return trace.Wrap(teleport.BadParameter("config", "please supply both TLS key and certificate"))
	}

	if len(cfg.AuthServers) == 0 {
		return trace.Wrap(teleport.BadParameter("proxy", "please supply a proxy server"))
	}

	return nil
}

// initSelfSignedHTTPSCert generates and self-signs a TLS key+cert pair for https connection
// to the proxy server.
func initSelfSignedHTTPSCert(cfg *Config) (err error) {
	log.Warningf("[CONFIG] NO TLS Keys provided, using self signed certificate")

	keyPath := filepath.Join(cfg.DataDir, defaults.SelfSignedKeyPath)
	certPath := filepath.Join(cfg.DataDir, defaults.SelfSignedCertPath)
	pubPath := filepath.Join(cfg.DataDir, defaults.SelfSignedPubPath)

	cfg.Proxy.TLSKey = keyPath
	cfg.Proxy.TLSCert = certPath

	// return the existing pair if they ahve already been generated:
	_, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return trace.Wrap(err, "unrecognized error reading certs")
	}
	log.Warningf("[CONFIG] Generating self signed key and cert to %v %v", keyPath, certPath)

	creds, err := utils.GenerateSelfSignedCert([]string{cfg.Hostname, "localhost"})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := ioutil.WriteFile(keyPath, creds.PrivateKey, 0600); err != nil {
		return trace.Wrap(err, "error writing key PEM")
	}
	if err := ioutil.WriteFile(certPath, creds.Cert, 0600); err != nil {
		return trace.Wrap(err, "error writing key PEM")
	}
	if err := ioutil.WriteFile(pubPath, creds.PublicKey, 0600); err != nil {
		return trace.Wrap(err, "error writing pub key PEM")
	}
	return nil
}

type FanOutEventLogger struct {
	Loggers []lunk.EventLogger
}

func (f *FanOutEventLogger) Log(id lunk.EventID, e lunk.Event) {
	for _, l := range f.Loggers {
		l.Log(id, e)
	}
}
