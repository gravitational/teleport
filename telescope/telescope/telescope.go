package main

import (
	"fmt"
	"os"

	"github.com/gravitational/teleport/auth"
	authority "github.com/gravitational/teleport/auth/native"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/backend/encryptedbk"
	"github.com/gravitational/teleport/backend/etcdbk"
	"github.com/gravitational/teleport/service"
	"github.com/gravitational/teleport/telescope/telescope/srv"
	"github.com/gravitational/teleport/tun"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
)

type Config struct {
	Log         *string
	LogSeverity *string

	TunAddr      utils.NetAddr
	WebAddr      utils.NetAddr
	AuthHTTPAddr utils.NetAddr
	AuthSSHAddr  utils.NetAddr

	DataDir *string

	FQDN        *string
	Domain      *string
	AssetsDir   *string
	CPAssetsDir *string

	TLSKeyFile  *string
	TLSCertFile *string

	Backend              *string
	BackendConfig        *string
	BackendEncryptionKey *string
}

func main() {
	cfg := Config{}

	app := kingpin.New("telescope", "Telescope is a service that receives inbound connections from remote teleport clusters and provides access to them")

	cfg.Log = app.Flag("log", "Log output, currently 'console' or 'syslog'").Default("console").String()
	cfg.LogSeverity = app.Flag("log-severity", "Log severity, INFO or WARN or ERROR").Default("WARN").String()
	cfg.FQDN = app.Flag("fqdn", "Telescope host FQDN").String()
	cfg.Domain = app.Flag("domain", "Telescope auth domain").String()

	app.Flag("auth-http-addr", "Telescope auth server address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "unix",
				Addr:    "/tmp/telescope.auth.sock",
			}, &cfg.AuthHTTPAddr),
	)

	app.Flag("auth-ssh-addr", "Telescope auth ssh server address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "localhost:33008",
			}, &cfg.AuthSSHAddr),
	)

	app.Flag("tun-addr", "Telescope tun agent dial address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "localhost:33006",
			}, &cfg.TunAddr),
	)

	app.Flag("web-addr", "Web address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "localhost:33007",
			}, &cfg.WebAddr),
	)

	cfg.DataDir = app.Flag("data-dir", "Data directory").Required().String()
	cfg.AssetsDir = app.Flag("assets-dir", "Assets directory").Default(".").String()
	cfg.CPAssetsDir = app.Flag("cp-assets-dir", "Control panel assets directory").Default(".").String()
	cfg.Backend = app.Flag("backend", "Auth backend type, currently only 'etcd'").Default("etcd").String()
	cfg.BackendConfig = app.Flag("backend-config", "Auth backend-specific configuration string").String()
	cfg.BackendEncryptionKey = app.Flag("backend-key", "If key file is provided, backend will be encrypted with that key").Default("").String()
	cfg.TLSKeyFile = app.Flag("tls-key", "TLS private key filename").String()
	cfg.TLSCertFile = app.Flag("tls-cert", "TLS Certificate filename").String()

	kingpin.MustParse(app.Parse(os.Args[1:]))

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
	sev, err := log.SeverityFromString(*cfg.LogSeverity)
	if err != nil {
		return nil, err
	}
	log.Init([]*log.LogConfig{&log.LogConfig{Name: *cfg.Log}})
	log.SetSeverity(sev)

	log.Infof("starting with config:\n %#v", cfg)

	if *cfg.DataDir == "" {
		return nil, fmt.Errorf("please supply data directory")
	}

	b, err := initBackend(*cfg.Backend,
		*cfg.BackendConfig, *cfg.BackendEncryptionKey)
	if err != nil {
		log.Errorf("failed to initialize backend: %v", err)
		return nil, err
	}

	asrv, hostSigner, err := auth.Init(
		b, authority.New(), *cfg.FQDN, *cfg.Domain, *cfg.DataDir)
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
				AssetsDir:   *s.cfg.AssetsDir,
				CPAssetsDir: *s.cfg.CPAssetsDir,
				AuthAddr:    s.cfg.AuthSSHAddr,
				FQDN:        *s.cfg.FQDN})
		if err != nil {
			log.Errorf("failed to launch web server: %v", err)
			return err
		}
		log.Infof("telescope web server starting")

		if (*s.cfg.TLSCertFile != "") && (*s.cfg.TLSKeyFile != "") {
			err := srv.ListenAndServeTLS(
				wsrv.Server.Addr,
				wsrv.Handler,
				*s.cfg.TLSCertFile,
				*s.cfg.TLSKeyFile,
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

func initBackend(btype, bcfg, encryptionKeyFile string) (backend.Backend, error) {
	var bk backend.Backend
	var err error

	switch btype {
	case "etcd":
		bk, err = etcdbk.FromString(bcfg)
	case "bolt":
		bk, err = boltbk.FromString(bcfg)
	default:
		return nil, fmt.Errorf("unsupported backend type: %v", btype)
	}
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	if len(encryptionKeyFile) == 0 {
		return bk, nil
	} else {
		encryptedBk, err := encryptedbk.New(bk, []string{encryptionKeyFile})
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
		return encryptedBk, nil
	}
}
