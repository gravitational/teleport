/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	kindcluster "sigs.k8s.io/kind/pkg/cluster"
	kindlog "sigs.k8s.io/kind/pkg/log"
)

type kubeCluster struct {
	log            *slog.Logger
	name           string
	kubeconfigPath string
	provider       *kindcluster.Provider
}

func (k *kubeCluster) start() error {
	if err := os.MkdirAll(filepath.Dir(k.kubeconfigPath), 0o755); err != nil {
		return err
	}

	k.provider = kindcluster.NewProvider(
		kindcluster.ProviderWithDocker(),
		kindcluster.ProviderWithLogger(kindSlogLogger{log: k.log}),
	)
	k.log.Info("starting kube cluster", "name", k.name)

	if err := k.provider.Delete(k.name, k.kubeconfigPath); err != nil {
		return fmt.Errorf("failed to delete stale kube cluster %s: %w", k.name, err)
	}
	if err := k.provider.Create(
		k.name,
		kindcluster.CreateWithKubeconfigPath(k.kubeconfigPath),
	); err != nil {
		return fmt.Errorf("creating kube cluster %q: %w", k.name, err)
	}

	k.log.Info("kube cluster is ready", "name", k.name, "kubeconfig", k.kubeconfigPath)
	return nil
}

func (k *kubeCluster) stop() {
	if k.provider == nil {
		return
	}

	k.log.Info("deleting kube cluster", "name", k.name)
	if err := k.provider.Delete(k.name, k.kubeconfigPath); err != nil {
		k.log.Warn("failed to delete kube cluster", "name", k.name, "error", err)
	}

	if err := os.Remove(k.kubeconfigPath); err != nil && !os.IsNotExist(err) {
		k.log.Warn("failed to remove kubeconfig", "path", k.kubeconfigPath, "error", err)
	}
}

type kindSlogLogger struct {
	log *slog.Logger
}

func (k kindSlogLogger) Warn(message string) {
	k.log.Warn(message)
}

func (k kindSlogLogger) Warnf(format string, args ...interface{}) {
	k.log.Warn(fmt.Sprintf(format, args...))
}

func (k kindSlogLogger) Error(message string) {
	k.log.Error(message)
}

func (k kindSlogLogger) Errorf(format string, args ...interface{}) {
	k.log.Error(fmt.Sprintf(format, args...))
}

func (k kindSlogLogger) V(level kindlog.Level) kindlog.InfoLogger {
	return kindSlogInfoLogger{
		log:   k.log,
		level: level,
	}
}

type kindSlogInfoLogger struct {
	log   *slog.Logger
	level kindlog.Level
}

func (k kindSlogInfoLogger) Enabled() bool {
	return k.log.Enabled(nil, k.slogLevel())
}

func (k kindSlogInfoLogger) Info(message string) {
	k.log.Log(nil, k.slogLevel(), message)
}

func (k kindSlogInfoLogger) Infof(format string, args ...interface{}) {
	k.Info(fmt.Sprintf(format, args...))
}

func (k kindSlogInfoLogger) slogLevel() slog.Level {
	if k.level == 0 {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}
