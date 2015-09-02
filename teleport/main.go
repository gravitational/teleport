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

	sshCmd := app.Command("ssh", "SSH server endpoint")

	sshCmd.Flag("ssh-addr", "SSH endpoint listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33001",
			}, &cfg.SSH.Addr),
	)

	cfg.SSH.Shell = sshCmd.Flag("ssh-shell", "path to shell to launch for interactive sessions").Default("/bin/bash").String()
	cfg.SSH.Token = sshCmd.Flag("ssh-token", "one time provisioning token for SSH node to register with authority").OverrideDefaultFromEnvar("TELEPORT_SSH_TOKEN").String()

	// Auth server role options
	authCmd := app.Command("auth", "Authentication server endpoint")
	cfg.Auth.Backend = authCmd.Flag("auth-backend", "auth backend type, 'etcd' or 'bolt'").Default("etcd").String()
	cfg.Auth.BackendConfig = authCmd.Flag("auth-backend-config", "auth backend-specific configuration string").String()
	cfg.Auth.EventBackend = authCmd.Flag("auth-event-backend", "event backend type, currently only 'bolt'").Default("bolt").String()
	cfg.Auth.EventBackendConfig = authCmd.Flag("auth-event-backend-config", "event backend-specific configuration string").String()
	cfg.Auth.RecordBackend = authCmd.Flag("auth-record-backend", "event backend type, currently only 'bolt'").Default("bolt").String()
	cfg.Auth.RecordBackendConfig = authCmd.Flag("auth-record-backend-config", "event backend-specific configuration string").String()

	authCmd.Flag("auth-http-addr", "Auth Server HTTP API listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "unix",
				Addr:    "/tmp/teleport.auth.sock",
			}, &cfg.Auth.HTTPAddr),
	)

	authCmd.Flag("auth-ssh-addr", "Auth Server SSH tunnel API listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33000",
			}, &cfg.Auth.SSHAddr),
	)

	cfg.Auth.Domain = authCmd.Flag("auth-domain", "authentication server domain name, e.g. example.com").String()

	// CP role options
	cpCmd := app.Command("cp", "Control Panel endpoint")
	cfg.CP.AssetsDir = cpCmd.Flag("cp-assets-dir", "path to control panel assets").String()

	cpCmd.Flag("cp-addr", "CP server web listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33002",
			}, &cfg.CP.Addr),
	)

	cfg.CP.Domain = cpCmd.Flag("cp-domain", "control panel domain to serve, e.g. example.com").String()

	// Outbound tunnel role options
	tunCmd := app.Command("tun", "Outbound tunnel")

	tunCmd.Flag("tun-srv-addr", "tun agent dial address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33006",
			}, &cfg.Tun.SrvAddr),
	)

	cfg.Tun.Token = tunCmd.Flag("tun-token", "one time provisioning token for tun agent to register with authority").OverrideDefaultFromEnvar("TELEPORT_TUN_TOKEN").String()

	selectedCmds := parseSeveralCmds(app, os.Args[1:])
	for _, cmd := range selectedCmds {
		switch cmd {
		case "auth":
			cfg.Auth.Enabled = true
		case "ssh":
			cfg.SSH.Enabled = true
		case "cp":
			cfg.CP.Enabled = true
		case "tun":
			cfg.Tun.Enabled = true
		}
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

func parseSeveralCmds(app *kingpin.Application, args []string) (selectedCmds []string) {
	selectedCmds = []string{}
	if len(args) == 0 {
		kingpin.MustParse(app.Parse(args))
	}
	for _, arg := range args {
		if arg == "help" || arg == "--help" {
			kingpin.MustParse(app.Parse(args))
		}
	}
	firstCmd := 0
	for args[firstCmd][0] == "-"[0] {
		firstCmd++
		if firstCmd >= len(args) {
			kingpin.MustParse(app.Parse(args))
		}
	}
	cmdStart := firstCmd
	cmdEnd := cmdStart
	for {
		if cmdStart >= len(args) {
			return
		}
		cmdEnd++
		for cmdEnd < len(args) && args[cmdEnd][0] == "-"[0] {
			cmdEnd++
		}
		selectedCmds = append(selectedCmds, args[cmdStart])
		kingpin.MustParse(app.Parse(
			append(args[0:firstCmd], args[cmdStart:cmdEnd]...),
		))
		cmdStart = cmdEnd
	}

}
