package main

import (
	"fmt"

	"net/http"
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/memlog"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/oxy/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/auth/openssh"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/etcdbk"
	"github.com/gravitational/teleport/cp"
	"github.com/gravitational/teleport/srv"
	"github.com/gravitational/teleport/sshutils"
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

		cli.BoolFlag{Name: "authSrv", Usage: "start CA authority server"},
		cli.StringFlag{Name: "authAddr", Value: "localhost:2023", Usage: "CA Auth controller server host:port pair to listen on"},
		cli.StringFlag{Name: "authTunAddr", Value: "localhost:2024", Usage: "CA Auth controller server SSH host:port pair to listen on"},
		cli.StringFlag{Name: "authKey", Usage: "CA auth encryption key"},

		cli.BoolFlag{Name: "cpSrv", Usage: "start Control Plane web server"},
		cli.StringFlag{Name: "cpAddr", Value: "localhost:2025", Usage: "Control Plane webserver"},
		cli.StringSliceFlag{Name: "cpAuth", Usage: "list of auth servers for CP to connect to", Value: &cli.StringSlice{"localhost:2024"}},
		cli.StringFlag{Name: "cpHost", Value: "localhost", Usage: "Control Plane webserver base host"},

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

	ml := memlog.New()

	hostSigner, err := sshutils.NewHostSigner(hostKey, hostCert)
	if err != nil {
		return err
	}

	elog := &FanOutEventLogger{
		Loggers: []lunk.EventLogger{
			lunk.NewTextEventLogger(log.GetLogger().Writer(log.SeverityInfo)),
			lunk.NewJSONEventLogger(ml),
		}}

	s, err := srv.New(
		sshutils.Addr{Net: "tcp", Addr: c.String("addr")},
		[]ssh.Signer{hostSigner},
		b,
		srv.SetShell(c.String("shell")),
		srv.SetEventLogger(elog),
	)
	if err != nil {
		return err
	}

	log.Infof("teleport ssh starting on %v", c.String("addr"))
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

			key, err := secret.EncodedStringToKey(c.String("authKey"))
			if err != nil {
				log.Errorf("invalid key %v", err)
				return
			}
			scrt, err := secret.New(&secret.Config{KeyBytes: key})
			if err != nil {
				log.Errorf("failed to start secret service: %v", err)
				return
			}

			asrv := auth.NewAuthServer(b, openssh.New(), scrt)
			tsrv, err := auth.NewTunServer(
				sshutils.Addr{Net: "tcp", Addr: c.String("authTunAddr")},
				[]ssh.Signer{hostSigner},
				"http://"+c.String("authAddr"),
				asrv)
			if err != nil {
				log.Errorf("failed to start teleport ssh tunnel")
				return
			}
			if err := tsrv.Start(); err != nil {
				log.Errorf("failed to start teleport ssh tunnel: %v", err)
				return
			}
			a := auth.NewAPIServer(asrv)
			t, err := trace.New(a, log.GetLogger().Writer(log.SeverityInfo))
			if err != nil {
				log.Fatalf("failed to start: %v", err)
			}

			log.Infof("teleport http authority starting on %v", authAddr)
			srv := &http.Server{
				Addr:    authAddr,
				Handler: t,
			}
			if err := srv.ListenAndServe(); err != nil {
				log.Fatalf("failed to start: %v", err)
			}
			log.Infof("teleport auth server exited gracefully")
		}()
	}

	if c.Bool("cpSrv") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			csrv, err := cp.NewServer(cp.Config{
				AuthSrv: c.StringSlice("cpAuth"),
				LogSrv:  ml,
				Host:    c.String("cpHost"),
			})
			if err != nil {
				log.Errorf("failed to start CP server: %v", err)
				return
			}
			cpAddr := c.String("cpAddr")
			log.Infof("teleport control panel starting on %v", cpAddr)
			srv := &http.Server{
				Addr:    cpAddr,
				Handler: csrv,
			}
			if err := srv.ListenAndServe(); err != nil {
				log.Fatalf("failed to start: %v", err)
			}
			log.Infof("teleport cp server exited gracefully")
		}()
	}

	wg.Wait()
	s.Wait()

	return nil
}

func initBackend(btype, bcfg string) (backend.Backend, error) {
	switch btype {
	case "etcd":
		return etcdbk.FromString(bcfg)
	}
	return nil, fmt.Errorf("unsupported backend type: %v", btype)
}

type FanOutEventLogger struct {
	Loggers []lunk.EventLogger
}

func (f *FanOutEventLogger) Log(id lunk.EventID, e lunk.Event) {
	for _, l := range f.Loggers {
		l.Log(id, e)
	}
}
