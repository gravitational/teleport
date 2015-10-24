package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/backend/etcdbk"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tun"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/telescope/telescope/srv"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	log.Initialize("console", "INFO")
	if err := run(); err != nil {
		log.Errorf("telescope error: %v", err)
		os.Exit(1)
	}
	log.Infof("telescope completed successfully")
}

func run() error {
	app := kingpin.New("telescope",
		"Telescope eceives connections from remote teleport clusters and provides access to them")

	configPath := app.Flag("config", "Path to a configuration file in YAML format").ExistingFile()
	useEnv := app.Flag("env", "Configure teleport from environment variables").Bool()

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return trace.Wrap(err)
	}

	var cfg Config
	if *useEnv {
		if err := service.ParseEnv(&cfg); err != nil {
			return trace.Wrap(err)
		}
	} else if *configPath != "" {
		if err := service.ParseYAMLFile(*configPath, &cfg); err != nil {
			return trace.Wrap(err)
		}
	} else {
		return trace.Errorf("Use either --config or --env flags, see --help for details")
	}

	kingpin.MustParse(app.Parse(os.Args[1:]))

	setDefaults(&cfg)

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
	err := log.Initialize(cfg.Log.Output, cfg.Log.Severity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.AssetsDir, err = filepath.Abs(cfg.AssetsDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.CPAssetsDir, err = filepath.Abs(cfg.CPAssetsDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.DataDir == "" {
		return nil, fmt.Errorf("please supply data directory")
	}

	log.Infof("starting with config:\n %#v", cfg)

	b, err := initBackend(&cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	asrv, hostSigner, err := auth.Init(auth.InitConfig{
		Backend:    b,
		FQDN:       cfg.FQDN,
		AuthDomain: cfg.Domain,
		DataDir:    cfg.DataDir,
		Authority:  authority.New(),
	})
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
	tsrv, err := tun.NewServer(s.cfg.TunAddr, []ssh.Signer{s.hs},
		auth.NewBackendAccessPoint(s.b))
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
		if err := utils.StartHTTPServer(s.cfg.Auth.HTTPAddr, asrv); err != nil {
			log.Errorf("failed to start server: %v", err)
			return err
		}
		return nil
	})

	// register auth SSH-based endpoint
	s.RegisterFunc(func() error {
		tsrv, err := auth.NewTunServer(
			s.cfg.Auth.SSHAddr, []ssh.Signer{s.hs},
			s.cfg.Auth.HTTPAddr,
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
				AuthAddr:    s.cfg.Auth.SSHAddr,
				FQDN:        s.cfg.FQDN})
		if err != nil {
			log.Errorf("failed to launch web server: %v", err)
			return err
		}
		log.Infof("telescope web server starting")

		if (s.cfg.TLSCert != "") && (s.cfg.TLSKey != "") {
			log.Infof("found TLS credentials, init TLS listeners")
			err := utils.ListenAndServeTLS(
				wsrv.Server.Addr,
				wsrv.Handler,
				s.cfg.TLSCert,
				s.cfg.TLSKey,
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
