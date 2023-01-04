package common

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	waitNoResolveDefaultPeriod  = "10s"
	waitNoResolveDefaultTimeout = "10m"
)

type waitFlags struct {
	duration time.Duration
	domain   string
	period   time.Duration
	timeout  time.Duration
}

func wait(flags waitFlags) {
	// We get parameters from environment variables
	utils.InitLogger(utils.LoggingForCLI, log.DebugLevel)
	ctx, _ := signal.NotifyContext(
		context.Background(), syscall.SIGTERM, syscall.SIGINT, os.Interrupt,
	)

	if flags.duration != 0 {
		err := waitDuration(ctx, flags.duration)
		if err != nil {
			log.Fatal(err)
		}
	}
	if flags.domain != "" {
		err := waitNoResolve(ctx, flags.domain, flags.period, flags.timeout)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func waitDuration(ctx context.Context, duration time.Duration) error {
	if duration == 0 {
		return trace.BadParameter("no duration provided")
	}
	timeoutCtx, _ := context.WithTimeout(ctx, duration)

	<-timeoutCtx.Done()

	err := timeoutCtx.Err()
	if !errors.Is(err, context.DeadlineExceeded) {
		return trace.Wrap(err)
	}
	return nil
}

func waitNoResolve(ctx context.Context, domain string, period, timeout time.Duration) error {
	if domain == "" {
		return trace.BadParameter("no domain provided")
	}

	if period == 0 {
		return trace.BadParameter("no period provided")
	}

	if timeout == 0 {
		return trace.BadParameter("no timeout provided")
	}

	var err error
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// We resolve the previous auth service until there's no IP returned.
	// This means all pods got rollout, and we don't risk connecting to
	// an auth pod running the previous version
	periodic := interval.New(interval.Config{
		Duration:      period,
		FirstDuration: time.Millisecond,
		Jitter:        retryutils.NewSeventhJitter(),
	})
	defer periodic.Stop()

	exit := false
	for !exit {
		select {
		case <-ctx.Done():
			// Context has been canceled, either we reached the timeout
			// or something else happened to the parent context
			err = ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				return trace.LimitExceeded(
					"timeout (%s) reached, but domain '%s' is still resolving",
					timeout,
					domain,
				)
			}
			return trace.Wrap(err)

		case <-periodic.Next():
			exit, err = checkDomainNoResolve(domain)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	log.Info("no endpoints found, exiting with success code")
	return nil
}

func checkDomainNoResolve(domainName string) (exit bool, err error) {
	endpoints, err := countEndpoints(domainName)
	if err != nil {
		dnsErr, ok := err.(*net.DNSError)
		if !ok {
			log.Errorf("unexpected error when resolving domain %s : %s", domainName, err)
			return false, trace.Wrap(err)
		}
		if dnsErr.Temporary() {
			log.Warnf("temporary error when resolving domain %s : %s", domainName, err)
			return false, nil
		}
		if dnsErr.IsNotFound {
			log.Infof("domain %s not found", domainName)
			return true, nil
		}
		log.Errorf("error when resolving domain %s : %s", domainName, err)
		return false, nil
	}
	log.Infof("%d endpoints found when resolving domain %s", endpoints, domainName)
	return endpoints == 0, nil
}

func countEndpoints(serviceName string) (int, error) {
	ips, err := net.LookupIP(serviceName)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return len(ips), nil
}
