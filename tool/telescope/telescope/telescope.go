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

type Config struct {
	Log service.LogConfig `yaml:"log"`

	// DataDir is address where telescope stores it's state
	// like user databases, etcd state and so on
	DataDir string `yaml:"data_dir" env:"TELESCOPE_DATA_DIR"`

	// FQDN is fully qualified domain name for telescope server
	FQDN string `yaml:"fqdn" env:"TELESCOPE_FQDN"`

	// TunAddr is address where telescope exposes listening socket for it's
	// bi-directional reverse tunnel (when teleports connect to it)
	TunAddr utils.NetAddr `yaml:"tun_addr" env:"TELESCOPE_TUN_ADDR"`
	// WebAddr is address for web portal of telescope
	WebAddr utils.NetAddr `yaml:"web_addr" env:"TELESCOPE_WEB_ADDR"`

	// AuthHTTPAddr is address for telescope's auth server to expose it's HTTP API
	AuthHTTPAddr utils.NetAddr `yaml:"auth_http_addr" env:"TELESCOPE_AUTH_HTTP_ADDR"`
	// AuthSSHAddr is addresss for telescope to expose it's HTTP API over SSH tunnel
	AuthSSHAddr utils.NetAddr `yaml:"auth_ssh_addr" env:"TELESCOPE_AUTH_SSH_ADDR"`

	// Domain is domain name for telescope SSH authority
	Domain string `yaml:"domain" env:"TELESCOPE_DOMAIN"`
	// AssetsDir is a directory with telescope's website assets
	AssetsDir string `yaml:"assets_dir" env:"TELESCOPE_ASSETS_DIR"`
	// CPAssetsDir is a directory with teleport's website assets that
	// are used by telescope
	CPAssetsDir string `yaml:"cp_assets_dir" env:"TELESCOPE_CP_ASSETS_DIR"`

	// TLSKey is a base64 encoded private key used by web portal
	TLSKey string `yaml:"tls_key" env:"TELESCOPE_TLS_KEY"`
	// TLSCert is a base64 encoded certificate used by web portal
	TLSCert string `yaml:"tlscert" env:"TELESCOPE_TLS_CERT"`

	// KeysBackend configures backend that stores encryption keys
	KeysBackend struct {
		// Type is a backend type - etcd or boltdb
		Type string `yaml:"type" env:"TELESCOPE_KEYS_BACKEND_TYPE"`
		// Params is map with backend specific parameters
		Params service.KeyVal `yaml:"params,flow" env:"TELESCOPE_KEYS_BACKEND_PARAMS"`
		// AdditionalKey is a additional signing GPG key
		AdditionalKey string `yaml:"additional_key" env:"TELESCOPE_KEYS_BACKEND_ADDITIONAL_KEY"`
	} `yaml:"keys_backend"`
}

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

func setDefaults(cfg *Config) {
	if cfg.Log.Output == "" {
		cfg.Log.Output = "console"
	}
	if cfg.Log.Severity == "" {
		cfg.Log.Severity = "INFO"
	}
	if cfg.AuthHTTPAddr.IsEmpty() {
		cfg.AuthHTTPAddr = utils.NetAddr{
			Network: "unix",
			Addr:    "/tmp/telescope.auth.sock",
		}
	}
	if cfg.AuthSSHAddr.IsEmpty() {
		cfg.AuthSSHAddr = utils.NetAddr{
			Network: "tcp",
			Addr:    "127.0.0.1:33008",
		}
	}
	if cfg.TunAddr.IsEmpty() {
		cfg.TunAddr = utils.NetAddr{
			Network: "tcp",
			Addr:    "127.0.0.1:33006",
		}
	}
	if cfg.WebAddr.IsEmpty() {
		cfg.WebAddr = utils.NetAddr{
			Network: "tcp",
			Addr:    "127.0.0.1:33007",
		}
	}
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

func initBackend(cfg *Config) (*encryptedbk.ReplicatedBackend, error) {
	var bk backend.Backend
	var err error

	switch cfg.KeysBackend.Type {
	case "etcd":
		bk, err = etcdbk.FromObject(cfg.KeysBackend.Params)
	case "bolt":
		bk, err = boltbk.FromObject(cfg.KeysBackend.Params)
	default:
		err = trace.Errorf("unsupported backend type: %v", cfg.KeysBackend.Type)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyStorage := path.Join(cfg.DataDir, "tscope_backend_keys")
	addKeys := []encryptor.Key{}
	if len(cfg.KeysBackend.AdditionalKey) != 0 {
		addKey, err := encryptedbk.LoadKeyFromFile(
			cfg.KeysBackend.AdditionalKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		addKeys = append(addKeys, addKey)
	}
	encryptedBk, err := encryptedbk.NewReplicatedBackend(
		bk, keyStorage, addKeys, encryptor.GenerateGPGKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return encryptedBk, nil
}
