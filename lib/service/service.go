package service

import (
	"net/http"
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
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	oxytrace "github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/oxy/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
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
	setDefaults(&cfg)

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
		if err := InitAuthService(supervisor, cfg.RoleConfig()); err != nil {
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
func InitAuthService(supervisor Supervisor, cfg RoleConfig) error {
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
	acfg := auth.InitConfig{
		Backend:            b,
		Authority:          authority.New(),
		FQDN:               cfg.Hostname,
		AuthDomain:         cfg.Auth.HostAuthorityDomain,
		DataDir:            cfg.DataDir,
		SecretKey:          cfg.Auth.SecretKey,
		AllowedTokens:      cfg.Auth.AllowedTokens,
		TrustedAuthorities: convertRemoteCerts(cfg.Auth.TrustedAuthorities),
	}
	if len(cfg.Auth.UserCA.PublicKey) != 0 && len(cfg.Auth.UserCA.PrivateKey) != 0 {
		acfg.UserCA = cfg.Auth.UserCA.ToCA()
	}
	if len(cfg.Auth.HostCA.PublicKey) != 0 && len(cfg.Auth.HostCA.PrivateKey) != 0 {
		acfg.HostCA = cfg.Auth.HostCA.ToCA()
	}
	asrv, signer, err := auth.Init(acfg)
	if err != nil {
		return trace.Wrap(err)
	}
	// register HTTP API endpoint
	supervisor.RegisterFunc(func() error {
		log.Infof("[AUTH] server HTTP endpoint is starting on %v", cfg.Auth.HTTPAddr)
		apisrv := auth.NewAPIServer(asrv, elog, session.New(b), rec)
		t, err := oxytrace.New(apisrv, log.GetLogger().Writer(log.SeverityInfo))
		if err != nil {
			return trace.Wrap(err)
		}
		return utils.StartHTTPServer(cfg.Auth.HTTPAddr, t)
	})

	// register auth SSH-based endpoint
	supervisor.RegisterFunc(func() error {
		log.Infof("[AUTH] server SSH endpoint is starting")
		tsrv, err := auth.NewTunServer(
			cfg.Auth.SSHAddr, []ssh.Signer{signer},
			cfg.Auth.HTTPAddr,
			asrv)
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
	return RegisterWithAuthServer(supervisor, cfg.SSH.Token, cfg.RoleConfig(), func() error {
		return initSSHEndpoint(supervisor, cfg)
	})
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
			lunk.NewTextEventLogger(log.GetLogger().Writer(log.SeverityInfo)),
			client,
		},
	}

	s, err := srv.New(cfg.SSH.Addr,
		[]ssh.Signer{signer},
		client,
		srv.SetShell(cfg.SSH.Shell),
		srv.SetEventLogger(elog),
		srv.SetSessionServer(client),
		srv.SetRecorder(client))
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

	// we are on the same server as the auth endpoint
	// and there's no token. we can handle this
	if cfg.Auth.Enabled && provisioningToken == "" {
		log.Infof("registering in embedded mode, connecting to local auth server")
		clt, err := auth.NewClientFromNetAddr(cfg.Auth.HTTPAddr)
		if err != nil {
			log.Errorf("failed to instantiate client: %v", err)
			return trace.Wrap(err)
		}
		provisioningToken, err = clt.GenerateToken(cfg.Hostname, 30*time.Second)
		if err != nil {
			log.Errorf("failed to generate token: %v", err)
		}
		return trace.Wrap(err)
	}
	supervisor.RegisterFunc(func() error {
		log.Infof("teleport:register connecting to auth servers %v", provisioningToken)
		if err := auth.Register(
			cfg.Hostname, cfg.DataDir, provisioningToken, cfg.AuthServers); err != nil {
			log.Errorf("teleport:ssh register failed: %v", err)
			return trace.Wrap(err)
		}
		log.Infof("teleport:register registered successfully")
		return callback()
	})
	return nil
}

func initReverseTunnel(supervisor Supervisor, cfg Config) error {
	return RegisterWithAuthServer(
		supervisor, cfg.ReverseTunnel.Token, cfg.RoleConfig(), func() error {
			return initTunAgent(supervisor, cfg)
		})
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
			lunk.NewTextEventLogger(log.GetLogger().Writer(log.SeverityInfo)),
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
		supervisor, cfg.Proxy.Token, cfg.RoleConfig(),
		func() error {
			return initProxyEndpoint(supervisor, cfg)
		})
}

func initProxyEndpoint(supervisor Supervisor, cfg Config) error {
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
		cfg.Proxy.ReverseTunnelListenAddr, []ssh.Signer{signer}, client)
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
				Tun:       tsrv,
				AssetsDir: cfg.Proxy.AssetsDir,
				AuthAddr:  cfg.AuthServers[0],
				FQDN:      cfg.Hostname})
		if err != nil {
			log.Errorf("failed to launch web server: %v", err)
			return err
		}

		if (cfg.Proxy.TLSCert != "") && (cfg.Proxy.TLSKey != "") {
			log.Infof("[PROXY] found TLS credentials, init TLS listeners")
			err := utils.ListenAndServeTLS(
				cfg.Proxy.WebAddr.Addr,
				webHandler,
				cfg.Proxy.TLSCert,
				cfg.Proxy.TLSKey)
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			err := http.ListenAndServe(cfg.Proxy.WebAddr.Addr, webHandler)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})

	return nil
}

func initBackend(dataDir, fqdn string, peers NetAddrSlice, cfg AuthConfig) (*encryptedbk.ReplicatedBackend, error) {
	var bk backend.Backend
	var err error

	switch cfg.KeysBackend.Type {
	case "etcd":
		bk, err = etcdbk.FromObject(cfg.KeysBackend.Params)
	case "bolt":
		bk, err = boltbk.FromObject(cfg.KeysBackend.Params)
	default:
		return nil, trace.Errorf("unsupported backend type: %v", cfg.KeysBackend.Type)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyStorage := path.Join(dataDir, "backend_keys")
	addKeys := []encryptor.Key{}
	if len(cfg.KeysBackend.AdditionalKey) != 0 {
		addKey, err := encryptedbk.LoadKeyFromFile(cfg.KeysBackend.AdditionalKey)
		if err != nil {
			return nil, err
		}
		addKeys = append(addKeys, addKey)
	}

	encryptedBk, err := encryptedbk.NewReplicatedBackend(bk,
		keyStorage, addKeys, encryptor.GenerateGPGKey)

	if err != nil {
		log.Errorf(err.Error())
		log.Infof("Initializing backend as follower node")
		myKey, err := encryptor.GenerateGPGKey(fqdn + " key")
		if err != nil {
			return nil, err
		}
		masterKey, err := auth.RegisterNewAuth(
			fqdn, cfg.Token, myKey.Public(), peers)
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

func initEventBackend(btype string, params interface{}) (events.Log, error) {
	switch btype {
	case "bolt":
		return boltlog.FromObject(params)
	}
	return nil, trace.Errorf("unsupported backend type: %v", btype)
}

func initRecordBackend(btype string, params interface{}) (recorder.Recorder, error) {
	switch btype {
	case "bolt":
		return boltrec.FromObject(params)
	}
	return nil, trace.Errorf("unsupported backend type: %v", btype)
}

func initLogging(ltype, severity string) error {
	return log.Initialize(ltype, severity)
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
