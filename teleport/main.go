package main

import (
	"fmt"

	"net/http"
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/auth/openssh"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/etcdbk"
	"github.com/gravitational/teleport/srv"
	"github.com/gravitational/teleport/utils"
	"sync"
)

func main() {
	app := cli.NewApp()
	app.Name = "teleport"
	app.Usage = "Clustering SSH and key management server"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "addr", Value: "localhost:2022", Usage: "SSH host:port pair to listen on"},
		cli.StringFlag{Name: "shell", Value: "/bin/sh", Usage: "path to shell to launch for interactive sessions"},
		cli.StringFlag{Name: "hostPrivateKey", Usage: "path to host SSH private key"},
		cli.StringFlag{Name: "hostCert", Usage: "path to host SSH signed certificate"},
		cli.StringFlag{Name: "backend", Value: "etcd", Usage: "backend type, currently only 'etcd'"},
		cli.StringFlag{Name: "backendConfig", Value: "", Usage: "backend-specific configuration string"},
		cli.BoolFlag{Name: "authSrv", Usage: "whether to start CA authority HTTP controller server"},
		cli.StringFlag{Name: "authAddr", Value: "localhost:2023", Usage: "CA Auth controller server host:port pair to listen on"},

		cli.StringFlag{Name: "log", Value: "console", Usage: "Log output, currently 'console' or 'syslog'"},
		cli.StringFlag{Name: "logSeverity", Value: "WARN", Usage: "Log severity, logs warning by default"},
	}
	app.Action = run
	app.Run(os.Args)
}

func run(c *cli.Context) {
	if err := parseArgs(c); err != nil {
		log.Errorf("failed to parse arguments, err %v", err)
	}
}

func setupLogging(c *cli.Context) error {
	s, err := log.SeverityFromString(c.String("logSeverity"))
	if err != nil {
		return err
	}
	log.Init([]*log.LogConfig{&log.LogConfig{Name: c.String("log")}})
	log.SetSeverity(s)
	return nil
}

func parseArgs(c *cli.Context) error {
	if err := setupLogging(c); err != nil {
		return err
	}
	hostKey, err := utils.ReadPath(c.String("hostPrivateKey"))
	if err != nil {
		return err
	}
	hostCert, err := utils.ReadPath(c.String("hostCert"))
	if err != nil {
		return err
	}
	b, err := initBackend(c.String("backend"), c.String("backendConfig"))
	if err != nil {
		return err
	}

	cfg := srv.Config{
		Addr:        c.String("addr"),
		HostCert:    hostCert,
		HostKey:     hostKey,
		Backend:     b,
		Shell:       c.String("shell"),
		EventLogger: lunk.NewTextEventLogger(log.GetLogger().Writer(log.SeverityInfo)),
	}
	s, err := srv.New(cfg)
	if err != nil {
		return err
	}

	log.Infof("teleport ssh starting on %v", cfg.Addr)
	if err := s.Start(); err != nil {
		log.Fatalf("failed to start: %v", err)
	}
	// TODO(klizhentas): cleanup this code and remove this logic to supervisor
	wg := &sync.WaitGroup{}
	if c.Bool("authSrv") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			authAddr := c.String("authAddr")
			log.Infof("teleport http authority starting on %v", authAddr)
			srv := auth.NewAPIServer(auth.NewAuthServer(b, openssh.New()))
			hsrv := &http.Server{
				Addr:    authAddr,
				Handler: srv,
			}
			if err := hsrv.ListenAndServe(); err != nil {
				log.Fatalf("failed to start: %v", err)
			}
			log.Infof("teleport auth server exited gracefully")
		}()
	}
	wg.Wait()

	return nil
}

func initBackend(btype, bcfg string) (backend.Backend, error) {
	switch btype {
	case "etcd":
		return etcdbk.FromString(bcfg)
	}
	return nil, fmt.Errorf("unsupported backend type: %v", btype)
}
