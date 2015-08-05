package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/service"
	"github.com/gravitational/teleport/utils"
)

func main() {
	cfg := service.Config{}

	flag.StringVar(
		&cfg.Log, "log", "console",
		"log output, currently 'console' or 'syslog'")

	flag.StringVar(
		&cfg.LogSeverity, "logSeverity", "WARN",
		"log severity, INFO or WARN or ERROR")

	flag.StringVar(
		&cfg.DataDir, "dataDir", "",
		"path to directory where teleport stores it's state")

	flag.StringVar(
		&cfg.FQDN, "fqdn", "",
		"fqdn of this server, e.g. node1.example.com, should be unique")

	flag.Var(utils.NewNetAddrList(&cfg.AuthServers),
		"authServer", "list of SSH auth server endpoints")

	// SSH specific role options
	flag.BoolVar(&cfg.SSH.Enabled, "ssh", false,
		"enable SSH server endpoint")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33001",
			}, &cfg.SSH.Addr),
		"sshAddr", "SSH endpoint listening address")

	flag.StringVar(
		&cfg.SSH.Shell, "sshShell", "/bin/bash",
		"path to shell to launch for interactive sessions")

	// Auth server role options
	flag.BoolVar(&cfg.Auth.Enabled, "auth", false,
		"enable Authentication server endpoint")

	flag.StringVar(
		&cfg.Auth.Backend, "authBackend", "etcd",
		"auth backend type, 'etcd' or 'bolt'")

	flag.StringVar(
		&cfg.Auth.BackendConfig, "authBackendConfig", "",
		"auth backend-specific configuration string")

	flag.StringVar(
		&cfg.Auth.EventBackend, "authEventBackend", "bolt",
		"event backend type, currently only 'bolt'")

	flag.StringVar(
		&cfg.Auth.EventBackendConfig, "authEventBackendConfig", "",
		"event backend-specific configuration string")

	flag.StringVar(
		&cfg.Auth.RecordBackend, "authRecordBackend", "bolt",
		"event backend type, currently only 'bolt'")

	flag.StringVar(
		&cfg.Auth.RecordBackendConfig, "authRecordBackendConfig", "",
		"event backend-specific configuration string")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "unix",
				Addr:    "/tmp/teleport.auth.sock",
			}, &cfg.Auth.HTTPAddr),
		"authHTTPAddr", "Auth Server HTTP API listening address")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33000",
			}, &cfg.Auth.SSHAddr),
		"authSSHAddr", "Auth Server SSH tunnel API listening address")

	flag.StringVar(
		&cfg.Auth.Domain, "authDomain", "",
		"authentication server domain name, e.g. example.com")

	flag.StringVar(
		&cfg.SSH.Token, "sshToken", "",
		"one time provisioning token for SSH node to register with authority")

	// CP role options
	flag.BoolVar(&cfg.CP.Enabled, "cp", false,
		"enable Control Panel endpoint")

	flag.StringVar(
		&cfg.CP.AssetsDir, "cpAssetsDir", "",
		"path to control panel assets")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33002",
			}, &cfg.CP.Addr),
		"cpAddr", "CP server web listening address")

	flag.StringVar(
		&cfg.CP.Domain, "cpDomain", "",
		"control panel domain to serve, e.g. example.com")

	// Outbound tunnel role options
	flag.BoolVar(&cfg.Tun.Enabled, "tun", false, "enable outbound tunnel")

	flag.Var(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33006",
			}, &cfg.Tun.SrvAddr),
		"tunSrvAddr", "tun agent dial address")

	flag.StringVar(
		&cfg.Tun.Token, "tunToken", "",
		"one time provisioning token for tun agent to register with authority")

	flag.Parse()

	// some variables can be set via environment variables
	// TODO(klizhentas) - implement
	if os.Getenv("TELEPORT_SSH_TOKEN") != "" {
		cfg.SSH.Token = os.Getenv("TELEPORT_SSH_TOKEN")
	}

	if os.Getenv("TELEPORT_TUN_TOKEN") != "" {
		cfg.Tun.Token = os.Getenv("TELEPORT_TUN_TOKEN")
	}

	srv, err := service.NewTeleport(cfg)
	if err != nil {
		fmt.Printf("error starting teleport: %v\n", err)
		return
	}

	if err := srv.Start(); err != nil {
		log.Errorf("teleport failed to start with error: %v", err)
		return
	}
	srv.Wait()
}
