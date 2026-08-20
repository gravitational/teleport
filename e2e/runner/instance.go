package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type testInstance struct {
	browser            string
	log                *slog.Logger
	proxyPort          int
	authPort           int
	sshPorts           []int
	kubePort           int
	e2eDir             string
	dataDir            string
	tctlBin            string
	noResourceSetup    bool
	suiteDir           string
	teleportConfigPath string
	teleport           *teleportInstance
	nodes              []*dockerNode
	kube               *kubeCluster
}

// start starts the Teleport instance and SSH node for this test instance.
// Called lazily from the playwright runner so that at most 2 instances run concurrently.
func (inst *testInstance) start(ctx context.Context) error {
	var err error

	defer func() {
		if err != nil {
			inst.stop()
		}
	}()

	if inst.kube != nil {
		if err = inst.kube.start(ctx); err != nil {
			return fmt.Errorf("failed to start kube fixture for %s: %w", inst.browser, err)
		}
	}

	if inst.teleport != nil {
		if err = inst.teleport.start(ctx); err != nil {
			return fmt.Errorf("failed to start Teleport for %s: %w", inst.browser, err)
		}
		if err = inst.teleport.waitReady(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("teleport for %s failed to become ready: %w", inst.browser, err)
		}
		if err = inst.teleport.seedRecordings(ctx, inst.e2eDir, inst.suiteDir, inst.dataDir); err != nil {
			return fmt.Errorf("failed to seed session recordings for %s: %w", inst.browser, err)
		}
		if !inst.noResourceSetup {
			if err = applyResources(ctx, inst.e2eDir, inst.suiteDir, inst.tctlBin, inst.teleportConfigPath); err != nil {
				return fmt.Errorf("failed to apply resources for %s: %w", inst.browser, err)
			}
		}
	}

	for _, node := range inst.nodes {
		if err = node.start(ctx); err != nil {
			return fmt.Errorf("failed to start docker node for %s: %w", inst.browser, err)
		}
		if err = node.waitJoined(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("docker node for %s failed to join cluster: %w", inst.browser, err)
		}
	}

	return nil
}

func (inst *testInstance) stop() {
	for _, node := range inst.nodes {
		node.stop(context.Background())
	}

	if inst.kube != nil {
		inst.kube.stop()
	}

	if inst.teleport != nil {
		inst.teleport.stop()
	}
}

var browserColors = map[string]string{
	"chromium": "\033[36m", // cyan
	"firefox":  "\033[33m", // yellow
	"webkit":   "\033[35m", // magenta
	"connect":  "\033[32m", // green
}

type prefixHandler struct {
	slog.Handler
	browser string
}

func (h *prefixHandler) Handle(ctx context.Context, r slog.Record) error {
	color, ok := browserColors[h.browser]
	if !ok {
		color = "\033[37m"
	}

	r.Message = fmt.Sprintf("%s[%s]\033[0m %s", color, h.browser, r.Message)

	return h.Handler.Handle(ctx, r)
}

func newBrowserLogger(browser string) *slog.Logger {
	return slog.New(&prefixHandler{
		Handler: slog.Default().Handler(),
		browser: browser,
	})
}
