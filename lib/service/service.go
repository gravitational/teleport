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
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport"
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
	AuthServers []utils.NetAddr
	Auth        AuthConfig
	Console     io.Writer
}

// NewTeleport takes the daemon configuration, instantiates all required services
// and starts them under a supervisor, returning the supervisor object
func NewTeleport(cfg Config) (Supervisor, error) {
	if err := validateConfig(&cfg); err != nil {
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

	// if there are no certificates, use self signed
	supervisor := NewSupervisor()

	if cfg.Auth.Enabled {
		if err := InitAuthService(supervisor, cfg.RoleConfig(), cfg.HostUUID); err != nil {
			return nil, err
		}
	}

	if cfg.SSH.Enabled {
		if err := initSSH(supervisor, cfg); err != nil {
			return nil, err
		}
	}

	if cfg.ReverseTunnel.Enabled {
		if err := initReverseTunnel(supervisor, cfg); err != nil {
			return nil, err
		}
	}

	if cfg.Proxy.Enabled {
		if err := initProxy(supervisor, cfg); err != nil {
			return nil, err
		}
	}

	return supervisor, nil
}

// InitAuthService can be called to initialize auth server service
func InitAuthService(supervisor Supervisor, cfg RoleConfig, hostUUID string) error {
	// Initialize the storage back-ends for keys, events and records
	b, err := initAuthStorage(cfg.DataDir, hostUUID, cfg.AuthServers, cfg.Auth)
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
	// configure the auth service:
	acfg := auth.InitConfig{
		Backend:       b,
		Authority:     authority.New(),
		DomainName:    cfg.HostUUID,
		DataDir:       cfg.DataDir,
		SecretKey:     cfg.Auth.SecretKey,
		AllowedTokens: cfg.Auth.AllowedTokens,
	}
	asrv, signer, err := auth.Init(acfg)
	if err != nil {
		return trace.Wrap(err)
	}

	sess, err := session.New(b)
	if err != nil {
		return trace.Wrap(err)
	}
	apisrv := auth.NewAPIWithRoles(asrv, elog, sess, rec,
		auth.NewStandardPermissions(), auth.StandardRoles,
	)
	supervisor.RegisterFunc(func() error {
		apisrv.Serve()
		return nil
	})

	limiter, err := limiter.NewLimiter(cfg.Auth.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	// Register an SSH endpoint which is used to create an SSH tunnel to send HTTP
	// requests to the Auth API
	supervisor.RegisterFunc(func() error {
		utils.Consolef(cfg.Console, "[AUTH]  Auth service is starting on %v", cfg.Auth.SSHAddr.Addr)
		tsrv, err := auth.NewTunnel(
			cfg.Auth.SSHAddr, []ssh.Signer{signer},
			apisrv,
			asrv,
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
	return nil
}

func initSSH(supervisor Supervisor, cfg Config) error {
	return RegisterWithAuthServer(supervisor, cfg.SSH.Token,
		cfg.RoleConfig(), teleport.RoleNode,
		func() error {
			return initSSHEndpoint(supervisor, cfg)
		},
	)
}

func initSSHEndpoint(supervisor Supervisor, cfg Config) error {
	i, err := auth.ReadIdentity(cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	endpointUser := i.Cert.ValidPrincipals[0]

	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		endpointUser,
		[]ssh.AuthMethod{ssh.PublicKeys(i.KeySigner)})
	if err != nil {
		return trace.Wrap(err)
	}

	limiter, err := limiter.NewLimiter(cfg.SSH.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("valid principals: %v", i.Cert.ValidPrincipals[0])
	s, err := srv.New(cfg.SSH.Addr,
		i.Cert.ValidPrincipals[0],
		[]ssh.Signer{i.KeySigner},
		client,
		cfg.DataDir,
		cfg.SSH.AdvertiseIP,
		srv.SetLimiter(limiter),
		srv.SetShell(cfg.SSH.Shell),
		srv.SetEventLogger(client),
		srv.SetSessionServer(client),
		srv.SetRecorder(client),
		srv.SetLabels(cfg.SSH.Labels, cfg.SSH.CmdLabels),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	supervisor.RegisterFunc(func() error {
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
func RegisterWithAuthServer(
	supervisor Supervisor,
	provisioningToken string,
	cfg RoleConfig,
	role teleport.Role,
	callback func() error) error {
	if cfg.DataDir == "" {
		return trace.Errorf("please supply data directory")
	}
	if len(cfg.AuthServers) == 0 {
		return trace.Errorf("supply at least one auth server")
	}

	// check host SSH keys
	haveKeys, err := auth.HaveHostKeys(cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}
	if haveKeys {
		return callback()
	}

	authServer := cfg.AuthServers[0].Addr

	// see if we've registered with this auth server before
	_, err = auth.ReadIdentity(cfg.DataDir)
	previouslyRegistered := (err == nil)

	// this means the server has not been initialized yet, we are starting
	// the registering client that attempts to connect to the auth server
	// and provision the keys
	supervisor.RegisterFunc(func() error {
		registered := previouslyRegistered && provisioningToken == ""
		if registered {
			log.Infof("already a member of the cluster with the auth server: %v", authServer)
		} else {
			for {
				log.Infof("joining the cluster with a token %v", provisioningToken)
				err := auth.Register(cfg.HostUUID, cfg.DataDir, provisioningToken, role, cfg.AuthServers)
				if err != nil {
					log.Errorf("[SSH] failed to join the cluster: %v", err)
					time.Sleep(time.Second * 5)
				} else {
					break
				}
			}
			utils.Consolef(os.Stdout, "[SSH] Successfully registered with the auth server %v", authServer)
		}
		return callback()
	})
	return nil
}

func initReverseTunnel(supervisor Supervisor, cfg Config) error {
	return RegisterWithAuthServer(
		supervisor, cfg.ReverseTunnel.Token, cfg.RoleConfig(),
		teleport.RoleNode,
		func() error {
			return initTunAgent(supervisor, cfg)
		},
	)
}

func initTunAgent(supervisor Supervisor, cfg Config) error {
	i, err := auth.ReadIdentity(cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	endpointUser := i.Cert.ValidPrincipals[0]

	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		endpointUser,
		[]ssh.AuthMethod{ssh.PublicKeys(i.KeySigner)})
	if err != nil {
		return trace.Wrap(err)
	}

	a, err := reversetunnel.NewAgent(
		cfg.ReverseTunnel.DialAddr,
		cfg.Hostname,
		[]ssh.Signer{i.KeySigner},
		client,
		reversetunnel.SetEventLogger(client))
	if err != nil {
		return trace.Wrap(err)
	}

	supervisor.RegisterFunc(func() error {
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
func initProxy(supervisor Supervisor, cfg Config) (err error) {
	// if no TLS key was provided for the web UI, generate a self signed cert
	if cfg.Proxy.TLSKey == "" {
		cfg.Proxy.TLSKey, cfg.Proxy.TLSCert, err = initSelfSignedHTTPSCert(&cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return RegisterWithAuthServer(
		supervisor, cfg.Proxy.Token, cfg.RoleConfig(), teleport.RoleNode,
		func() error {
			return initProxyEndpoint(supervisor, cfg)
		},
	)
}

func initProxyEndpoint(supervisor Supervisor, cfg Config) error {

	proxyLimiter, err := limiter.NewLimiter(cfg.Proxy.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	reverseTunnelLimiter, err := limiter.NewLimiter(cfg.ReverseTunnel.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	i, err := auth.ReadIdentity(cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	endpointUser := i.Cert.ValidPrincipals[0]
	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		endpointUser,
		[]ssh.AuthMethod{ssh.PublicKeys(i.KeySigner)})
	if err != nil {
		return trace.Wrap(err)
	}

	tsrv, err := reversetunnel.NewServer(
		cfg.Proxy.ReverseTunnelListenAddr,
		[]ssh.Signer{i.KeySigner},
		client,
		reversetunnel.SetLimiter(reverseTunnelLimiter),
		reversetunnel.DirectSite(i.Cert.Extensions[utils.CertExtensionAuthority], client),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	SSHProxy, err := srv.New(cfg.Proxy.SSHAddr,
		cfg.Hostname,
		[]ssh.Signer{i.KeySigner},
		client,
		cfg.DataDir,
		nil,
		srv.SetLimiter(proxyLimiter),
		srv.SetProxyMode(tsrv),
		srv.SetSessionServer(client),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// register SSH reverse tunnel server that accepts connections
	// from remote teleport nodes
	supervisor.RegisterFunc(func() error {
		utils.Consolef(cfg.Console, "[PROXY] Reverse tunnel service is starting on %v", cfg.Proxy.ReverseTunnelListenAddr.Addr)
		if err := tsrv.Start(); err != nil {
			utils.Consolef(cfg.Console, "[PROXY] Error: %v", err)
			return trace.Wrap(err)
		}
		tsrv.Wait()
		return nil
	})

	// Register web proxy server
	supervisor.RegisterFunc(func() error {
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
	supervisor.RegisterFunc(func() error {
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
func initAuthStorage(dataDir, hostUUID string, peers NetAddrSlice, cfg AuthConfig) (*encryptedbk.ReplicatedBackend, error) {
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
		myKey, err := encryptor.GenerateGPGKey(hostUUID + " key")
		if err != nil {
			return nil, err
		}
		masterKey, err := auth.RegisterNewAuth(hostUUID,
			cfg.Token, myKey.Public(), peers)
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

	return nil
}

// initSelfSignedHTTPSCert generates and self-signs a TLS key+cert pair for https connection
// to the proxy server.
func initSelfSignedHTTPSCert(cfg *Config) (keyPath string, certPath string, err error) {
	log.Warningf("[CONFIG] NO TLS Keys provided, using self signed certificate")
	keyPath = filepath.Join(cfg.DataDir, selfSignedKeyPath)
	certPath = filepath.Join(cfg.DataDir, selfSignedCertPath)

	// return the existing pair if they ahve already been generated:
	_, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err == nil {
		return keyPath, certPath, nil
	}
	if !os.IsNotExist(err) {
		return "", "", trace.Wrap(err, "unrecognized error reading certs")
	}
	log.Warningf("[CONFIG] Generating self signed key and cert to %v %v", keyPath, certPath)
	keyPEM, certPEM, err := utils.GenerateSelfSignedCert([]string{cfg.Hostname}, []string{"127.0.0.1"})
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	if err := ioutil.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return "", "", trace.Wrap(err, "error writing key PEM")
	}
	if err := ioutil.WriteFile(certPath, certPEM, 0600); err != nil {
		return "", "", trace.Wrap(err, "error writing key PEM")
	}
	return keyPath, certPath, nil
}

const (
	// path to a self-signed TLS key file for HTTPS connection for the web proxy
	selfSignedKeyPath = "webproxy_https.key"
	// path to a self-signed TLS cert file for HTTPS connection for the web proxy
	selfSignedCertPath = "webproxy_https.cert"
)

type FanOutEventLogger struct {
	Loggers []lunk.EventLogger
}

func (f *FanOutEventLogger) Log(id lunk.EventID, e lunk.Event) {
	for _, l := range f.Loggers {
		l.Log(id, e)
	}
}
