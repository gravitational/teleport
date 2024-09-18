package client

import (
	"context"
	"fmt"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"time"
)

type checkResult struct {
	Name           string
	Addr           string
	Error          error
	ConnectionTime time.Duration
}

func CheckClientConnectors(ctx context.Context, cfg Config) ([]checkResult, error) {
	answers := []checkResult{}
	connectors := []struct {
		Name string
		Func connectFunc
	}{
		{Name: "dialerConnect", Func: dialerConnect},
		{Name: "authConnect", Func: authConnect},
		{Name: "tunnelConnect", Func: tunnelConnect},
		{Name: "tlsRoutingConnect", Func: tlsRoutingConnect},
		{Name: "tlsRoutingWithConnUpgradeConnect", Func: tlsRoutingWithConnUpgradeConnect},
	}

	tlsConfig, err := cfg.Credentials[0].TLSConfig()
	if err != nil && !trace.IsNotImplemented(err) {
		return nil, err
	}
	sshClientConfig, err := cfg.Credentials[0].SSHClientConfig()
	credentialDialer, err := cfg.Credentials[0].Dialer(cfg)

	for _, addr := range cfg.Addrs {
		for _, connector := range connectors {
			logrus.Infof("Testing %s %s", addr, connector.Name)
			ctx, cancel := context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			start := time.Now()
			client, err := connector.Func(ctx, connectParams{
				cfg:       cfg,
				addr:      addr,
				dialer:    credentialDialer,
				tlsConfig: tlsConfig,
				sshConfig: sshClientConfig,
			})
			connectTime := time.Since(start)
			if err != nil {
				answers = append(answers, checkResult{
					Name:           connector.Name,
					Addr:           addr,
					Error:          fmt.Errorf("connect: %w", err),
					ConnectionTime: connectTime,
				})
				continue
			}

			// Send a proto.AuthService.Ping to confirm we are talking to the
			// correct gRPC service.
			_, err = client.Ping(ctx)
			if err != nil {
				answers = append(answers, checkResult{
					Name:           connector.Name,
					Addr:           addr,
					Error:          fmt.Errorf("ping: %w", err),
					ConnectionTime: connectTime,
				})
				if err := client.Close(); err != nil {
					panic(err)
				}
				continue
			}

			answers = append(answers, checkResult{
				Name:           connector.Name,
				Addr:           addr,
				ConnectionTime: connectTime,
			})
			if err := client.Close(); err != nil {
				panic(err)
			}
		}
	}

	return answers, nil
}
