package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gravitational/teleport/telescope/telescope/srv"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/auth"
	authority "github.com/gravitational/teleport/auth/native"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/backend/etcdbk"
	"github.com/gravitational/teleport/service"
	"github.com/gravitational/teleport/tun"
	"github.com/gravitational/teleport/utils"
)

type Config struct {
	Log         string
	LogSeverity string

	TunAddr      utils.NetAddr
	WebAddr      utils.NetAddr
	AuthHTTPAddr utils.NetAddr
	AuthSSHAddr  utils.NetAddr

	DataDir string

	FQDN        string
	Domain      string
	AssetsDir   string
	CPAssetsDir string

	TLSKeyFile  string
	TLSCertFile string

	Backend       string
	BackendConfig string
}

func main() {
	cfg := Config{}

	flag.StringVar(
		&cfg.Log, "log", "console",
		"log output, currently 'console' or 'syslog'")

	flag.StringVar(
		&cfg.LogSeverity, "logSeverity", "WARN",
		"log severity, INFO or WARN or ERROR")

	flag.StringVar(
		&cfg.FQDN, "fqdn", "", "telescope host fqdn")

	flag.StringVar(
		&cfg.Domain, "domain", "", "telescope auth domain")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "unix",
				Addr:    "/tmp/telescope.auth.sock",
			}, &cfg.AuthHTTPAddr),
		"authHTTPAddr", "auth server address")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "localhost:33008",
			}, &cfg.AuthSSHAddr),
		"authSSHAddr", "auth ssh server address")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "localhost:33006",
			}, &cfg.TunAddr),
		"tunAddr", "tun agent dial address")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "localhost:33007",
			}, &cfg.WebAddr),
		"webAddr", "web address")

	flag.StringVar(&cfg.DataDir, "dataDir", "", "data dir")

	flag.StringVar(
		&cfg.AssetsDir, "assetsDir", ".", "assets dir")

	flag.StringVar(
		&cfg.CPAssetsDir, "cpAssetsDir", ".", "assets dir")

	flag.StringVar(
		&cfg.Backend, "backend", "etcd",
		"auth backend type, currently only 'etcd'")

	flag.StringVar(
		&cfg.BackendConfig, "backendConfig", "",
		"auth backend-specific configuration string")

	flag.StringVar(
		&cfg.TLSKeyFile, "tlskey", "", "TLS private key filename")

	flag.StringVar(
		&cfg.TLSCertFile, "tlscert", "", "TLS Certificate filename")

	flag.Parse()

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(-1)
	}
}

func run(cfg Config) error {
	srv, err := NewService(cfg)
	if err != nil {
		return err
	}
	if err := srv.Start(); err != nil {
		return err
	}
	srv.Wait()
	return nil
}

func NewService(cfg Config) (*Service, error) {
	sev, err := log.SeverityFromString(cfg.LogSeverity)
	if err != nil {
		return nil, err
	}
	log.Init([]*log.LogConfig{&log.LogConfig{Name: cfg.Log}})
	log.SetSeverity(sev)

	log.Infof("starting with config:\n %#v", cfg)

	if cfg.DataDir == "" {
		return nil, fmt.Errorf("please supply data directory")
	}

	b, err := initBackend(cfg.Backend, cfg.BackendConfig)
	if err != nil {
		log.Errorf("failed to initialize backend: %v", err)
		return nil, err
	}

	asrv, hostSigner, err := auth.Init(
		b, authority.New(), cfg.FQDN, cfg.Domain, cfg.DataDir)
	if err != nil {
		log.Errorf("failed to init auth server: %v", err)
		return nil, err
	}

	s := &Service{cfg: cfg, b: b, a: asrv, hs: hostSigner}
	s.Supervisor = *service.New()
	if err := s.addStart(); err != nil {
		return nil, err
	}
	return s, nil
}

type Service struct {
	service.Supervisor
	b  backend.Backend
	a  *auth.AuthServer
	hs ssh.Signer

	cfg Config
}

func (s *Service) addStart() error {
	tsrv, err := tun.NewServer(s.cfg.TunAddr, []ssh.Signer{s.hs})
	if err != nil {
		log.Errorf("failed to start server: %v", err)
		return err
	}

	s.RegisterFunc(func() error {
		log.Infof("telescope tunnel server starting")
		if err := tsrv.Start(); err != nil {
			log.Errorf("failed to start: %v", err)
			return err
		}
		tsrv.Wait()
		return nil
	})

	asrv := auth.NewAPIServer(s.a, nil, nil, nil)

	// register Auth HTTP API endpoint
	s.RegisterFunc(func() error {
		log.Infof("telescope auth server starting")
		if err := utils.StartHTTPServer(s.cfg.AuthHTTPAddr, asrv); err != nil {
			log.Errorf("failed to start server: %v", err)
			return err
		}
		return nil
	})

	// register auth SSH-based endpoint
	s.RegisterFunc(func() error {
		tsrv, err := auth.NewTunServer(
			s.cfg.AuthSSHAddr, []ssh.Signer{s.hs},
			s.cfg.AuthHTTPAddr,
			s.a)
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

	// Register control panel web server
	s.RegisterFunc(func() error {
		wsrv, err := srv.New(s.cfg.WebAddr,
			srv.Config{
				Tun:         tsrv,
				AssetsDir:   s.cfg.AssetsDir,
				CPAssetsDir: s.cfg.CPAssetsDir,
				AuthAddr:    s.cfg.AuthSSHAddr,
				FQDN:        s.cfg.FQDN})
		if err != nil {
			log.Errorf("failed to launch web server: %v", err)
			return err
		}
		log.Infof("telescope web server starting")

		if (s.cfg.TLSCertFile != "") && (s.cfg.TLSKeyFile != "") {
			err := srv.ListenAndServeTLS(
				wsrv.Server.Addr,
				wsrv.Handler,
				s.cfg.TLSCertFile,
				s.cfg.TLSKeyFile,
			)
			if err != nil {
				log.Errorf("failed to start: %v", err)
				return err
			}
		} else {

			if err := wsrv.ListenAndServe(); err != nil {
				log.Errorf("failed to start: %v", err)
				return err
			}
		}
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
