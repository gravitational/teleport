package service

import (
	"fmt"

	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/auth/openssh"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/etcdbk"
	"github.com/gravitational/teleport/cp"
	"github.com/gravitational/teleport/srv"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/memlog"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/oxy/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type TeleportService struct {
	Supervisor
	log memlog.Logger
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
	t.log = memlog.New()

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

	return t, nil
}

func initAuth(t *TeleportService, cfg Config) error {
	a := cfg.Auth
	if a.Domain == "" {
		return fmt.Errorf("please provide auth domain, e.g. example.com")
	}

	b, err := initBackend(cfg.Auth.Backend, cfg.Auth.BackendConfig)
	if err != nil {
		return err
	}

	asrv, signer, err := auth.Init(
		b, openssh.New(), cfg.FQDN, cfg.Auth.Domain, cfg.DataDir)
	if err != nil {
		log.Errorf("failed to init auth server: %v", err)
		return err
	}

	// register HTTP API endpoint
	t.RegisterFunc(func() error {
		apisrv := auth.NewAPIServer(asrv)
		t, err := trace.New(apisrv, log.GetLogger().Writer(log.SeverityInfo))
		if err != nil {
			log.Fatalf("failed to start: %v", err)
		}

		log.Infof("teleport http authority starting on %v", a.HTTPAddr)
		return utils.StartHTTPServer(a.HTTPAddr, t)
	})

	// register SSH endpoint
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
		LogSrv:  t.log,
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
		return initSSHRegister(t, cfg)
	}
	return initSSHEndpoint(t, cfg)
}

func initSSHEndpoint(t *TeleportService, cfg Config) error {
	signer, err := auth.ReadKeys(cfg.FQDN, cfg.DataDir)

	elog := &FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.GetLogger().Writer(log.SeverityInfo)),
			lunk.NewJSONEventLogger(t.log),
		}}

	client, err := auth.NewTunClient(
		cfg.AuthServers[0],
		cfg.FQDN,
		[]ssh.AuthMethod{ssh.PublicKeys(signer)})

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

func initSSHRegister(t *TeleportService, cfg Config) error {
	t.RegisterFunc(func() error {
		log.Infof("teleport:ssh connecting to auth servers")
		if err := auth.Register(
			cfg.FQDN, cfg.DataDir, cfg.SSH.Token, cfg.AuthServers); err != nil {
			log.Errorf("teleport:ssh register failed: %v", err)
			return err
		}
		log.Infof("teleport:ssh registered successfully, starting SSH endpoint")
		return initSSHEndpoint(t, cfg)
	})
	return nil
}

func initBackend(btype, bcfg string) (backend.Backend, error) {
	switch btype {
	case "etcd":
		return etcdbk.FromString(bcfg)
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
	if cfg.DataDir == "" {
		return fmt.Errorf("please supply data directory")
	}
	if !cfg.Auth.Enabled && !cfg.SSH.Enabled && !cfg.CP.Enabled {
		return fmt.Errorf("supply at least one of Auth, SSH or CP roles")
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
