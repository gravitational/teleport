/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
	"time"
)

type teleportInstance struct {
	log         *slog.Logger
	teleportBin string
	proxyPort   int
	configPath  string
	stateFile   string
	logFile     string // empty means stdout/stderr

	cmd      *exec.Cmd
	logF     *os.File
	waitErr  error
	waitDone chan struct{}
}

func (t *teleportInstance) start(ctx context.Context) error {
	t.cmd = exec.CommandContext(ctx, t.teleportBin, "start", "-c", t.configPath, "--bootstrap", t.stateFile)

	if t.logFile != "" {
		f, err := os.Create(t.logFile)
		if err != nil {
			return fmt.Errorf("creating log file: %w", err)
		}
		t.logF = f
		t.cmd.Stdout = f
		t.cmd.Stderr = f
	} else {
		t.cmd.Stdout = os.Stdout
		t.cmd.Stderr = os.Stderr
	}

	t.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	t.log.Info("starting teleport with bootstrap state")
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("starting teleport: %w", err)
	}

	t.waitDone = make(chan struct{})
	go func() {
		t.waitErr = t.cmd.Wait()
		close(t.waitDone)
	}()

	return nil
}

// waitReady polls the proxy's /webapi/ping endpoint until it responds with 200.
func (t *teleportInstance) waitReady(ctx context.Context, timeout time.Duration) error {
	t.log.Debug("waiting for teleport to be ready")

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	defer client.CloseIdleConnections()

	probe := func(_ context.Context) (bool, error) {
		select {
		case <-t.waitDone:
			return false, fmt.Errorf("teleport exited unexpectedly: %w", t.waitErr)
		default:
		}

		resp, err := client.Get(fmt.Sprintf("https://localhost:%d/webapi/ping", t.proxyPort))
		if err != nil {
			return false, nil
		}

		resp.Body.Close()

		return resp.StatusCode == http.StatusOK, nil
	}

	if err := pollUntil(ctx, timeout, 1*time.Second, probe); err != nil {
		return fmt.Errorf("teleport failed to become ready: %w", err)
	}

	t.log.Info("teleport is ready")

	return nil
}

func (t *teleportInstance) stop() {
	if t.cmd == nil || t.cmd.Process == nil {
		return
	}

	t.log.Info("stopping teleport")

	select {
	case <-t.waitDone:
		// Already exited.
	default:
		_ = syscall.Kill(-t.cmd.Process.Pid, syscall.SIGTERM)

		select {
		case <-t.waitDone:
		case <-time.After(5 * time.Second):
			t.log.Warn("teleport did not exit gracefully, sending SIGKILL")
			_ = syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
			<-t.waitDone
		}
	}

	if t.logF != nil {
		t.logF.Close()
	}
}

type TeleportConfig struct {
	DataDir        string
	AuthServerPort int
	ProxyPort      int
	KeyFilePath    string
	CertFilePath   string
	LicenseFile    string
	LogLevel       string
}

func generateTeleportConfig(templatePath, outPath string, data *TeleportConfig) (string, error) {
	return renderTemplateToPath(templatePath, outPath, data)
}

type TeleportNodeConfig struct {
	AuthServerHost string
	AuthServerPort int
	SSHServerPort  int
}

func generateTeleportNodeConfig(templatePath, outPath string, data *TeleportNodeConfig) (string, error) {
	return renderTemplateToPath(templatePath, outPath, data)
}

func resolveDockerHost() (string, error) {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		return "host.docker.internal", nil
	}

	u, err := url.Parse(dockerHost)
	if err != nil {
		return "", fmt.Errorf("parsing DOCKER_HOST: %w", err)
	}

	conn, err := net.Dial("udp", u.Host)
	if err != nil {
		return "", fmt.Errorf("dialing docker host: %w", err)
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

type StateConfig struct {
	PasswordHashBase64  string
	CredentialIDBase64  string
	PublicKeyCBORBase64 string
}

func generateStateFile(templatePath string, creds *credentials) (string, error) {
	stateConfig := &StateConfig{
		PasswordHashBase64:  creds.passwordHashBase64,
		CredentialIDBase64:  creds.credentialIDBase64,
		PublicKeyCBORBase64: creds.publicKeyCBORBase64,
	}

	return renderTemplate(templatePath, stateConfig)
}

func renderTemplate(templatePath string, data any) (string, error) {
	return renderTemplateToPath(templatePath, strings.TrimSuffix(templatePath, ".tmpl"), data)
}

func renderTemplateToPath(templatePath, outPath string, data any) (string, error) {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", err
	}

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}

	if err := tmpl.Execute(f, data); err != nil {
		f.Close()
		return "", err
	}

	if err := f.Close(); err != nil {
		return "", err
	}

	return outPath, nil
}
