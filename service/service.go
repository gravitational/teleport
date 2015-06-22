package service

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/auth"
	authority "github.com/gravitational/teleport/auth/native"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/backend/etcdbk"
	"github.com/gravitational/teleport/cp"
	"github.com/gravitational/teleport/srv"
	"github.com/gravitational/teleport/tun"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/memlog"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/oxy/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type TeleportService struct {
	Supervisor
}

func NewTeleport(cfg Config) (*TeleportService, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	if err := initLogging(cfg.Log, cfg.LogSeverity); err != nil {
		return nil, err
	}

	t := &TeleportService{}
	t.Supervisor = *New()

	if cfg.Auth.Enabled {
		if err := initAuth(t, cfg); err != nil {
			return nil, err
		}
	}

	if cfg.CP.Enabled {
		if err := initCP(t, cfg); err != nil {
			return nil, err
		}
	}

	if cfg.SSH.Enabled {
		if err := initSSH(t, cfg); err != nil {
			return nil, err
		}
	}

	if cfg.Tun.Enabled {
		if err := initTun(t, cfg); err != nil {
			return nil, err
		}
	}

	return t, nil
}

func initAuth(t *TeleportService, cfg Config) error {
	if cfg.DataDir == "" {
		return fmt.Errorf("please supply data directory")
	}
	a := cfg.Auth
	if a.Domain == "" {
		return fmt.Errorf("please provide auth domain, e.g. example.com")
	}

	b, err := initBackend(cfg.Auth.Backend, cfg.Auth.BackendConfig)
	if err != nil {
		return err
	}

	asrv, signer, err := auth.Init(
		b, authority.New(), cfg.FQDN, cfg.Auth.Domain, cfg.DataDir)
	if err != nil {
		log.Errorf("failed to init auth server: %v", err)
		return err
	}

	// register HTTP API endpoint
	t.RegisterFunc(func() error {
		apisrv := auth.NewAPIServer(asrv, memlog.New())
		t, err := trace.New(apisrv, log.GetLogger().Writer(log.SeverityInfo))
		if err != nil {
			log.Fatalf("failed to start: %v", err)
		}

		log.Infof("teleport http authority starting on %v", a.HTTPAddr)
		return utils.StartHTTPServer(a.HTTPAddr, t)
	})

	// register auth SSH-based endpoint
	t.RegisterFunc(func() error {
		tsrv, err := auth.NewTunServer(
			a.SSHAddr, []ssh.Signer{signer},
			a.HTTPAddr,
			asrv)
		if err != nil {
			log.Errorf("failed to start teleport ssh tunnel")
			return err
		}
		if err := tsrv.Start(); err != nil {
			log.Errorf("failed to start teleport ssh endpoint: %v", err)
			return err
		}
		return nil
	})
	return nil
}

func initCP(t *TeleportService, cfg Config) error {
	if len(cfg.AuthServers) == 0 {
		return fmt.Errorf("supply at least one auth server")
	}
	if len(cfg.CP.Domain) == 0 {
		return fmt.Errorf("cp hostname is required")
	}
	csrv, err := cp.NewServer(cp.Config{
		AuthSrv: cfg.AuthServers,
		Host:    cfg.CP.Domain,
	})
	if err != nil {
		log.Errorf("failed to start CP server: %v", err)
		return err
	}
	log.Infof("teleport control panel starting on %v", cfg.CP.Addr)

	t.RegisterFunc(func() error {
		return utils.StartHTTPServer(cfg.CP.Addr, csrv)
	})
	return nil
}

func initSSH(t *TeleportService, cfg Config) error {
	if cfg.DataDir == "" {
		return fmt.Errorf("please supply data directory")
	}
	if len(cfg.AuthServers) == 0 {
		return fmt.Errorf("supply at least one auth server")
	}
	haveKeys, err := auth.HaveKeys(cfg.FQDN, cfg.DataDir)
	if err != nil {
		return err
	}
	if !haveKeys {
		// this means the server has not been initialized yet we are starting
		// the registering client that attempts to connect ot the auth server
		// and provision the keys
		return initRegister(t, cfg.SSH.Token, cfg)
	}
	return initSSHEndpoint(t, cfg)
}

func initSSHEndpoint(t *TeleportService, cfg Config) error {
	signer, err := auth.ReadKeys(cfg.FQDN, cfg.DataDir)

	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		cfg.FQDN,
		[]ssh.AuthMethod{ssh.PublicKeys(signer)})

	elog := &FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.GetLogger().Writer(log.SeverityInfo)),
			lunk.NewJSONEventLogger(client.GetLogWriter()),
		}}

	s, err := srv.New(cfg.SSH.Addr,
		[]ssh.Signer{signer},
		client,
		srv.SetShell(cfg.SSH.Shell),
		srv.SetEventLogger(elog))
	if err != nil {
		return err
	}

	t.RegisterFunc(func() error {
		log.Infof("teleport ssh starting on %v", cfg.SSH.Addr)
		if err := s.Start(); err != nil {
			log.Fatalf("failed to start: %v", err)
			return err
		}
		s.Wait()
		return nil
	})
	return nil
}

func initRegister(t *TeleportService, token string, cfg Config) error {
	// we are on the same server as the auth endpoint
	// and there's no token. we can handle this
	if cfg.Auth.Enabled && token == "" {
		log.Infof("registering in embedded mode, connecting to local auth server")
		clt, err := auth.NewClientFromNetAddr(cfg.Auth.HTTPAddr)
		if err != nil {
			log.Errorf("failed to instantiate client: %v", err)
			return err
		}
		token, err = clt.GenerateToken(cfg.FQDN, 30*time.Second)
		if err != nil {
			log.Errorf("failed to generate token: %v", err)
		}
		return err
	}
	t.RegisterFunc(func() error {
		log.Infof("teleport:register connecting to auth servers %v", cfg.SSH.Token)
		if err := auth.Register(
			cfg.FQDN, cfg.DataDir, token, cfg.AuthServers); err != nil {
			log.Errorf("teleport:ssh register failed: %v", err)
			return err
		}
		log.Infof("teleport:register registered successfully")
		return initSSHEndpoint(t, cfg)
	})
	return nil
}

func initTun(t *TeleportService, cfg Config) error {
	if cfg.DataDir == "" {
		return fmt.Errorf("please supply data directory")
	}
	if len(cfg.AuthServers) == 0 {
		return fmt.Errorf("supply at least one auth server")
	}
	haveKeys, err := auth.HaveKeys(cfg.FQDN, cfg.DataDir)
	if err != nil {
		return err
	}
	if !haveKeys {
		// this means the server has not been initialized yet we are starting
		// the registering client that attempts to connect ot the auth server
		// and provision the keys
		return initRegister(t, cfg.Tun.Token, cfg)
	}
	return initTunAgent(t, cfg)
}

func initTunAgent(t *TeleportService, cfg Config) error {
	signer, err := auth.ReadKeys(cfg.FQDN, cfg.DataDir)

	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		cfg.FQDN,
		[]ssh.AuthMethod{ssh.PublicKeys(signer)})

	elog := &FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.GetLogger().Writer(log.SeverityInfo)),
			lunk.NewJSONEventLogger(client.GetLogWriter()),
		}}

	a, err := tun.NewAgent(
		cfg.Tun.SrvAddr,
		cfg.FQDN,
		[]ssh.Signer{signer},
		client,
		tun.SetEventLogger(elog))
	if err != nil {
		return err
	}

	t.RegisterFunc(func() error {
		log.Infof("teleport ws agent starting")
		if err := a.Start(); err != nil {
			log.Fatalf("failed to start: %v", err)
			return err
		}
		a.Wait()
		return nil
	})
	return nil
}

func initBackend(btype, bcfg string) (backend.Backend, error) {
	switch btype {
	case "etcd":
		return etcdbk.FromString(bcfg)
	case "bolt":
		return boltbk.FromString(bcfg)
	}
	return nil, fmt.Errorf("unsupported backend type: %v", btype)
}

func initLogging(ltype, severity string) error {
	s, err := log.SeverityFromString(severity)
	if err != nil {
		return err
	}
	log.Init([]*log.LogConfig{&log.LogConfig{Name: ltype}})
	log.SetSeverity(s)
	return nil
}

func validateConfig(cfg Config) error {
	if !cfg.Auth.Enabled && !cfg.SSH.Enabled && !cfg.CP.Enabled && !cfg.Tun.Enabled {
		return fmt.Errorf("supply at least one of Auth, SSH, CP or Tun roles")
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

type Config struct {
	HostCertPath string
	HostKeyPath  string

	Log         string
	LogSeverity string

	DataDir string
	FQDN    string

	AuthServers []utils.NetAddr

	SSH  SSHConfig
	Auth AuthConfig
	CP   CPConfig
	Tun  TunConfig
}

type AuthConfig struct {
	Enabled       bool
	HTTPAddr      utils.NetAddr
	SSHAddr       utils.NetAddr
	Domain        string
	Backend       string
	BackendConfig string
}

type SSHConfig struct {
	Token   string
	Enabled bool
	Addr    utils.NetAddr
	Shell   string
}

type CPConfig struct {
	Enabled bool
	Addr    utils.NetAddr
	Domain  string
}

type TunConfig struct {
	Token   string
	Enabled bool
	SrvAddr utils.NetAddr
}
