package main

import (
	"fmt"
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/service"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	cfg := service.Config{}

	app := kingpin.New("teleport", "Teleport is a clustering SSH server and SSH certificate authority that provides audit logs, web access, command multiplexing and more.")
	cfg.Log = app.Flag("log", "log output, currently 'console' or 'syslog'").Default("console").String()
	cfg.LogSeverity = app.Flag("log-severity", "log severity, INFO or WARN or ERROR").Default("WARN").String()

	cfg.DataDir = app.Flag("data-dir", "path to directory where teleport stores it's state").Required().String()

	cfg.FQDN = app.Flag("fqdn", "fqdn of this server, e.g. node1.example.com, should be unique").String()

	app.Flag("auth-server", "list of SSH auth server endpoints").SetValue(
		utils.NewNetAddrList(&cfg.AuthServers),
	)

	cfg.SSH.Enabled = app.Flag("ssh", "enable SSH server endpoint").Default("false").Bool()

	app.Flag("ssh-addr", "SSH endpoint listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33001",
			}, &cfg.SSH.Addr),
	)

	cfg.SSH.Shell = app.Flag("ssh-shell", "path to shell to launch for interactive sessions").Default("/bin/bash").String()

	// Auth server role options
	cfg.Auth.Enabled = app.Flag("auth", "enable Authentication server endpoint").Default("false").Bool()
	cfg.Auth.Backend = app.Flag("auth-backend", "auth backend type, 'etcd' or 'bolt'").Default("etcd").String()
	cfg.Auth.BackendConfig = app.Flag("auth-backend-config", "auth backend-specific configuration string").String()
	cfg.Auth.EventBackend = app.Flag("auth-event-backend", "event backend type, currently only 'bolt'").Default("bolt").String()
	cfg.Auth.EventBackendConfig = app.Flag("auth-event-backend-config", "event backend-specific configuration string").String()
	cfg.Auth.RecordBackend = app.Flag("auth-record-backend", "event backend type, currently only 'bolt'").Default("bolt").String()
	cfg.Auth.RecordBackendConfig = app.Flag("auth-record-backend-config", "event backend-specific configuration string").String()

	app.Flag("auth-http-addr", "Auth Server HTTP API listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "unix",
				Addr:    "/tmp/teleport.auth.sock",
			}, &cfg.Auth.HTTPAddr),
	)

	app.Flag("auth-ssh-addr", "Auth Server SSH tunnel API listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33000",
			}, &cfg.Auth.SSHAddr),
	)

	cfg.Auth.Domain = app.Flag("auth-domain", "authentication server domain name, e.g. example.com").String()
	cfg.SSH.Token = app.Flag("ssh-token", "one time provisioning token for SSH node to register with authority").String()

	// CP role options
	cfg.CP.Enabled = app.Flag("cp", "enable Control Panel endpoint").Default("false").Bool()
	cfg.CP.AssetsDir = app.Flag("cp-assets-dir", "path to control panel assets").String()

	app.Flag("cp-addr", "CP server web listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33002",
			}, &cfg.CP.Addr),
	)

	cfg.CP.Domain = app.Flag("cp-domain", "control panel domain to serve, e.g. example.com").String()

	// Outbound tunnel role options
	cfg.Tun.Enabled = app.Flag("tun", "enable outbound tunnel").Default("false").Bool()

	app.Flag("tun-srv-addr", "tun agent dial address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33006",
			}, &cfg.Tun.SrvAddr),
	)

	cfg.Tun.Token = app.Flag("tun-token", "one time provisioning token for tun agent to register with authority").String()

	kingpin.MustParse(app.Parse(os.Args[1:]))

	// some variables can be set via environment variables
	// TODO(klizhentas) - implement
	if os.Getenv("TELEPORT_SSH_TOKEN") != "" {
		*cfg.SSH.Token = os.Getenv("TELEPORT_SSH_TOKEN")
	}

	if os.Getenv("TELEPORT_TUN_TOKEN") != "" {
		*cfg.Tun.Token = os.Getenv("TELEPORT_TUN_TOKEN")
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
