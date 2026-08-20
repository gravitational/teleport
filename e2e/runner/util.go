package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// pollUntil calls probe at the given interval until it returns true,
// the timeout expires, or the context is canceled.
func pollUntil(ctx context.Context, timeout, interval time.Duration, probe func(ctx context.Context) (bool, error)) error {
	deadline := time.After(timeout)
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-deadline:
			return fmt.Errorf("timed out after %s", timeout)

		case <-tick.C:
			ok, err := probe(ctx)
			if err != nil {
				return err
			}

			if ok {
				return nil
			}
		}
	}
}

func resolveE2EDir() (string, error) {
	if v := os.Getenv("E2E_DIR"); v != "" {
		return filepath.Abs(v)
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	e2eDir := filepath.Dir(filepath.Dir(exePath))
	return e2eDir, nil
}
