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
	// split args ino several commands
	commands := splitCmds(os.Args[1:])

	// create config for each command and then merge values to the
	// one final config

	// use the first command config as base
	cfg := getFilledConfig(commands[0], true)

	switch commands[0][0] {
	case "auth":
		cfg.Auth.Enabled = true
	case "ssh":
		cfg.SSH.Enabled = true
	case "cp":
		cfg.CP.Enabled = true
	case "tun":
		cfg.Tun.Enabled = true
	}

	// read the rest of the commands
	for _, cmd := range commands[1:] {
		curCfg := getFilledConfig(cmd, false)

		switch cmd[0] {
		case "auth":
			cfg.Auth = curCfg.Auth
			cfg.Auth.Enabled = true
		case "ssh":
			cfg.SSH = curCfg.SSH
			cfg.SSH.Enabled = true
		case "cp":
			cfg.CP = curCfg.CP
			cfg.CP.Enabled = true
		case "tun":
			cfg.Tun = curCfg.Tun
			cfg.Tun.Enabled = true
		}
	}

	// if user started auth and something else and did not
	// provide auth address for that something,
	// the address of the created auth will be used
	if cfg.Auth.Enabled && len(cfg.AuthServers) == 0 {
		cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}
	}

	srv, err := service.NewTeleport(cfg)
	if err != nil {
		fmt.Printf("error starting teleport: %v\n", err)
		os.Exit(-1)
	}

	if err := srv.Start(); err != nil {
		log.Errorf("teleport failed to start with error: %v", err)
		os.Exit(-1)
	}
	srv.Wait()
}

// getFilledConfig parses args and returns service.Config value
func getFilledConfig(args []string, commandIsFirst bool) service.Config {
	cfg := service.Config{}

	app := kingpin.New("teleport", "Teleport is a clustering SSH server and SSH certificate authority that provides audit logs, web access, command multiplexing and more.")

	// in case of several commands, all the general flags should be
	// provided before the commands
	if commandIsFirst {
		cfg.Log = app.Flag("log", "log output, currently 'console' or 'syslog'. Should be provided before commands.").Default("console").String()
		cfg.LogSeverity = app.Flag("log-severity", "log severity, INFO or WARN or ERROR. Should be provided before commands.").Default("WARN").String()

		cfg.DataDir = app.Flag("data-dir", "path to directory where teleport stores it's state. Should be provided before commands.").Required().String()

		cfg.FQDN = app.Flag("fqdn", "fqdn of this server, e.g. node1.example.com, should be unique. Should be provided before commands.").String()

		app.Flag("auth-server", "list of SSH auth server endpoints. Used for 'ssh', 'cp' and 'tun' commands. Should be provided before commands.").SetValue(
			utils.NewNetAddrList(&cfg.AuthServers),
		)
	}

	// SSH node options
	sshCmd := app.Command("ssh", "SSH server endpoint")

	sshCmd.Flag("addr", "SSH endpoint listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33001",
			}, &cfg.SSH.Addr),
	)

	cfg.SSH.Shell = sshCmd.Flag("shell", "path to shell to launch for interactive sessions").Default("/bin/bash").String()
	cfg.SSH.Token = sshCmd.Flag("token", "one time provisioning token for SSH node to register with authority").OverrideDefaultFromEnvar("TELEPORT_SSH_TOKEN").String()

	// Auth server role options
	authCmd := app.Command("auth", "Authentication server endpoint")

	cfg.Auth.Backend = authCmd.Flag("backend", "backend type, 'etcd' or 'bolt'").Default("etcd").String()
	cfg.Auth.BackendConfig = authCmd.Flag("backend-config", "backend-specific configuration string").Required().String()
	cfg.Auth.BackendEncryptionKey = authCmd.Flag("backend-key", "If key file is provided, backend will be encrypted with that key").Default("").String()
	cfg.Auth.EventBackend = authCmd.Flag("event-backend", "event backend type, currently only 'bolt'").Default("bolt").String()
	cfg.Auth.EventBackendConfig = authCmd.Flag("event-backend-config", "event backend-specific configuration string").Required().String()
	cfg.Auth.RecordBackend = authCmd.Flag("record-backend", "event backend type, currently only 'bolt'").Default("bolt").String()
	cfg.Auth.RecordBackendConfig = authCmd.Flag("record-backend-config", "event backend-specific configuration string").Required().String()

	authCmd.Flag("http-addr", "auth HTTP API listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "unix",
				Addr:    "/tmp/teleport.auth.sock",
			}, &cfg.Auth.HTTPAddr),
	)

	authCmd.Flag("ssh-addr", "auth SSH tunnel API listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33000",
			}, &cfg.Auth.SSHAddr),
	)

	cfg.Auth.Domain = authCmd.Flag("domain", "auth server domain name, e.g. example.com").String()

	// CP role options
	cpCmd := app.Command("cp", "Control Panel endpoint")
	cfg.CP.AssetsDir = cpCmd.Flag("assets-dir", "path to control panel assets").String()

	cpCmd.Flag("addr", "CP web server listening address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33002",
			}, &cfg.CP.Addr),
	)

	cfg.CP.Domain = cpCmd.Flag("domain", "control panel domain to serve, e.g. example.com").String()

	// Outbound tunnel role options
	tunCmd := app.Command("tun", "Outbound tunnel")

	tunCmd.Flag("srv-addr", "tun agent dial address").SetValue(
		utils.NewNetAddrVal(
			utils.NetAddr{
				Network: "tcp",
				Addr:    "127.0.0.1:33006",
			}, &cfg.Tun.SrvAddr),
	)

	cfg.Tun.Token = tunCmd.Flag("token", "one time provisioning token for tun agent to register with authority").OverrideDefaultFromEnvar("TELEPORT_TUN_TOKEN").String()

	_, err := app.Parse(args)
	if err != nil {
		fmt.Printf("Error: %s, try --help\n", err.Error())
		os.Exit(-1)
	}

	return cfg

}

// splitCmds split list of arguments to separate commands
// e.g. ["--f1", "--f2", "cmd1", "--f3", "--f4", "cmd2", "--f5"] ->
// ["--f1", "--f2", "cmd1", "--f3", "--f4"],
// ["cmd2", "f5"]
func splitCmds(args []string) (separatedCmds [][]string) {
	separatedCmds = [][]string{}
	if len(args) == 0 {
		return [][]string{args}
	}

	// if args contain 'help', just return them
	for _, arg := range args {
		if arg == "help" || arg == "--help" {
			return [][]string{args}
		}
	}

	// skip the general flags and find the first command in args
	firstCmd := 0
	for args[firstCmd][0] == '-' {
		firstCmd++
		if firstCmd >= len(args) {
			return [][]string{args}
		}
	}
	cmdStart := firstCmd
	cmdEnd := cmdStart
	for i := 0; true; i++ {
		// now both cmdStart and cmdEnd point to the beginning of the
		// current command
		if cmdStart >= len(args) {
			return separatedCmds
		}

		// find the beginning of the next command
		cmdEnd++
		for cmdEnd < len(args) && args[cmdEnd][0] == '-' {
			cmdEnd++
		}

		// merge general flags only with the first command
		var currentCmd []string
		if i == 0 {
			currentCmd = append(currentCmd, args[cmdStart:cmdEnd]...)
			currentCmd = append(currentCmd, args[0:firstCmd]...)
		} else {
			currentCmd = args[cmdStart:cmdEnd]
		}
		separatedCmds = append(separatedCmds, currentCmd)
		cmdStart = cmdEnd
	}

	return separatedCmds
}
