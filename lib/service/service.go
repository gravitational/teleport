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
package service

import (
	"io/ioutil"
	"log/syslog"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
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
	Hostname    string
	AuthServers []utils.NetAddr
	Auth        AuthConfig
}

func NewTeleport(cfg Config) (Supervisor, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	SetDefaults(&cfg)

	_, err := os.Stat(cfg.DataDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(cfg.DataDir, os.ModeDir|0777)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// if user started auth and something else and did not
	// provide auth address for that something,
	// the address of the created auth will be used
	if cfg.Auth.Enabled && len(cfg.AuthServers) == 0 {
		cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}
	}

	if err := initLogging(cfg.Log.Output, cfg.Log.Severity); err != nil {
		return nil, err
	}

	supervisor := NewSupervisor()

	if cfg.Auth.Enabled {
		if err := InitAuthService(supervisor, cfg.RoleConfig(), cfg.Hostname); err != nil {
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
func InitAuthService(supervisor Supervisor, cfg RoleConfig, hostname string) error {
	if cfg.Auth.HostAuthorityDomain == "" {
		return trace.Errorf(
			"please provide host certificate authority domain, e.g. example.com")
	}

	b, err := initBackend(cfg.DataDir, cfg.Hostname, cfg.AuthServers, cfg.Auth)
	if err != nil {
		return trace.Wrap(err)
	}

	elog, err := initEventBackend(
		cfg.Auth.EventsBackend.Type, cfg.Auth.EventsBackend.Params)
	if err != nil {
		return trace.Wrap(err)
	}

	rec, err := initRecordBackend(
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
	if len(cfg.Auth.UserCA.PublicKey) != 0 && len(cfg.Auth.UserCA.PrivateKey) != 0 {
		acfg.UserCA, err = cfg.Auth.UserCA.CA()
		if err != nil {
			return trace.Wrap(err)
		}
		acfg.UserCA.DomainName = hostname
		acfg.UserCA.Type = services.UserCert
	}
	if len(cfg.Auth.HostCA.PublicKey) != 0 && len(cfg.Auth.HostCA.PrivateKey) != 0 {
		acfg.HostCA, err = cfg.Auth.HostCA.CA()
		if err != nil {
			return trace.Wrap(err)
		}
		acfg.HostCA.DomainName = hostname
		acfg.HostCA.Type = services.HostCert
	}
	asrv, signer, err := auth.Init(acfg)
	if err != nil {
		return trace.Wrap(err)
	}
	apisrv := auth.NewAPIWithRoles(asrv, elog, session.New(b), rec,
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

	// register auth SSH-based endpoint
	supervisor.RegisterFunc(func() error {
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
		if err := tsrv.Start(); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	return nil
}

func initSSH(supervisor Supervisor, cfg Config) error {
	return RegisterWithAuthServer(supervisor, cfg.SSH.Token,
		cfg.RoleConfig(), auth.RoleNode,
		func() error {
			return initSSHEndpoint(supervisor, cfg)
		},
	)
}

func initSSHEndpoint(supervisor Supervisor, cfg Config) error {
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

	elog := &FanOutEventLogger{
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

	supervisor.RegisterFunc(func() error {
		log.Infof("[SSH] server is starting on %v", cfg.SSH.Addr)
		if err := s.Start(); err != nil {
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
	role string,
	callback func() error) error {
	if cfg.DataDir == "" {
		return trace.Errorf("please supply data directory")
	}
	if len(cfg.AuthServers) == 0 {
		return trace.Errorf("supply at least one auth server")
	}

	// check host SSH keys
	haveKeys, err := auth.HaveKeys(cfg.Hostname, cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}
	if haveKeys {
		return callback()
	}

	// this means the server has not been initialized yet, we are starting
	// the registering client that attempts to connect to the auth server
	// and provision the keys

	supervisor.RegisterFunc(func() error {
		log.Infof("teleport:register connecting to auth servers %v", provisioningToken)
		registered := false
		for !registered {
			if err := auth.Register(cfg.Hostname, cfg.DataDir,
				provisioningToken, role, cfg.AuthServers); err != nil {
				log.Errorf("teleport:ssh register failed: %v", err)
				time.Sleep(time.Second)
			} else {
				registered = true
			}
		}
		log.Infof("teleport:register registered successfully")
		return callback()
	})
	return nil
}

func initReverseTunnel(supervisor Supervisor, cfg Config) error {
	return RegisterWithAuthServer(
		supervisor, cfg.ReverseTunnel.Token, cfg.RoleConfig(),
		auth.RoleNode,
		func() error {
			return initTunAgent(supervisor, cfg)
		},
	)
}

func initTunAgent(supervisor Supervisor, cfg Config) error {
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

	elog := &FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.StandardLogger().Writer()),
			client,
		}}

	a, err := reversetunnel.NewAgent(
		cfg.ReverseTunnel.DialAddr,
		cfg.Hostname,
		[]ssh.Signer{signer},
		client,
		reversetunnel.SetEventLogger(elog))
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

func initProxy(supervisor Supervisor, cfg Config) error {
	return RegisterWithAuthServer(
		supervisor, cfg.Proxy.Token, cfg.RoleConfig(), auth.RoleNode,
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

	tsrv, err := reversetunnel.NewServer(
		cfg.Proxy.ReverseTunnelListenAddr,
		[]ssh.Signer{signer},
		client,
		reverseTunnelLimiter,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	SSHProxy, err := srv.New(cfg.Proxy.SSHAddr,
		cfg.Hostname,
		[]ssh.Signer{signer},
		client,
		proxyLimiter,
		cfg.DataDir,
		srv.SetProxyMode(tsrv),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// register SSH reverse tunnel server that accepts connections
	// from remote teleport nodes
	supervisor.RegisterFunc(func() error {
		log.Infof("[PROXY] reverse tunnel listening server starting on %v",
			cfg.Proxy.ReverseTunnelListenAddr)
		if err := tsrv.Start(); err != nil {
			return trace.Wrap(err)
		}
		tsrv.Wait()
		return nil
	})

	// Register web proxy server
	supervisor.RegisterFunc(func() error {
		log.Infof("[PROXY] teleport web proxy server starting on %v",
			cfg.Proxy.WebAddr.Addr)

		webHandler, err := web.NewMultiSiteHandler(
			web.MultiSiteConfig{
				Tun:        tsrv,
				AssetsDir:  cfg.Proxy.AssetsDir,
				AuthAddr:   cfg.AuthServers[0],
				DomainName: cfg.Hostname})
		if err != nil {
			log.Errorf("failed to launch web server: %v", err)
			return err
		}

		proxyLimiter.WrapHandle(webHandler)

		if (cfg.Proxy.TLSCert != "") && (cfg.Proxy.TLSKey != "") {
			log.Infof("[PROXY] found TLS credentials, init TLS listeners")
			err := utils.ListenAndServeTLS(
				cfg.Proxy.WebAddr.Addr,
				proxyLimiter,
				cfg.Proxy.TLSCert,
				cfg.Proxy.TLSKey)
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			err := http.ListenAndServe(cfg.Proxy.WebAddr.Addr, proxyLimiter)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})

	// Register ssh proxy server
	supervisor.RegisterFunc(func() error {
		log.Infof("[PROXY] teleport ssh proxy server starting on %v",
			cfg.Proxy.SSHAddr.Addr)
		if err := SSHProxy.Start(); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})

	return nil
}

func initBackend(dataDir, domainName string, peers NetAddrSlice, cfg AuthConfig) (*encryptedbk.ReplicatedBackend, error) {
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

// initLogging configures the logger according to config file values
func initLogging(ltype, severity string) error {
	useSyslog := true
	infoLevel := log.ErrorLevel

	// output
	switch strings.ToLower(ltype) {
	case "console", "stderr":
		useSyslog = false
	}

	if useSyslog {
		hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_ERR, "")
		if err != nil {
			return trace.Wrap(err)
		}
		log.AddHook(hook)
		log.SetOutput(ioutil.Discard)
	} else {
		log.SetOutput(os.Stderr)
	}

	// severity
	switch strings.ToLower(severity) {
	case "err", "error":
		infoLevel = log.ErrorLevel
	case "warn", "warning":
		infoLevel = log.WarnLevel
	case "info", "notice":
		infoLevel = log.InfoLevel
	case "fatal":
		infoLevel = log.FatalLevel
	}
	log.SetLevel(infoLevel)
	return nil
}

func validateConfig(cfg Config) error {
	if !cfg.Auth.Enabled && !cfg.SSH.Enabled && !cfg.ReverseTunnel.Enabled {
		return trace.Errorf("supply at least one of Auth, SSH or ReverseTunnel or Proxy roles")
	}

	if cfg.DataDir == "" {
		return trace.Errorf("please supply data directory")
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
