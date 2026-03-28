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
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/go-sdk/container"
	apicontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const nodeImage = "debian:bookworm-slim"

type dockerNode struct {
	log                *slog.Logger
	sshPort            int
	tctlBin            string
	teleportConfigPath string
	logFilePath        string
	nodeName           string

	imageName     string
	containerName string
	configPath    string
	teleportBin   string

	ctr *container.Container
}

func (d *dockerNode) start(ctx context.Context) error {
	return d.runContainer(ctx)
}

func (d *dockerNode) removeStale(ctx context.Context) {
	cli, err := client.New(client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()

	_, _ = cli.ContainerRemove(ctx, d.containerName, client.ContainerRemoveOptions{Force: true})
}

func (d *dockerNode) runContainer(ctx context.Context) error {
	d.log.Info("starting docker SSH node", "name", d.nodeName)

	d.removeStale(ctx)

	ctr, err := container.Run(ctx,
		container.WithImage(d.imageName),
		container.WithImagePlatform("linux/amd64"),
		container.WithPullHandler(func(r io.ReadCloser) error {
			_, err := io.Copy(io.Discard, r)
			return err
		}),
		container.WithName(d.containerName),
		container.WithEntrypoint("teleport", "start", "--insecure", "-c", "/etc/teleport/node.yaml"),
		container.WithExposedPorts(fmt.Sprintf("%d/tcp", d.sshPort)),
		container.WithFiles(
			container.File{
				HostPath:      d.teleportBin,
				ContainerPath: "/usr/local/bin/teleport",
				Mode:          0o755,
			},
			container.File{
				HostPath:      d.configPath,
				ContainerPath: "/etc/teleport/node.yaml",
				Mode:          0o644,
			},
		),
		container.WithHostConfigModifier(func(hc *apicontainer.HostConfig) {
			if os.Getenv("DOCKER_HOST") == "" {
				hc.ExtraHosts = []string{"host.docker.internal:host-gateway"}
			}
			hc.PortBindings = network.PortMap{
				network.MustParsePort(fmt.Sprintf("%d/tcp", d.sshPort)): []network.PortBinding{
					{HostPort: fmt.Sprintf("%d", d.sshPort)},
				},
			}
		}),
	)

	if err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	d.ctr = ctr

	return nil
}

func (d *dockerNode) waitJoined(ctx context.Context, timeout time.Duration) error {
	d.log.Debug("waiting for docker node to join cluster")

	probe := func(ctx context.Context) (bool, error) {
		cmd := exec.CommandContext(ctx, d.tctlBin, "nodes", "ls",
			"-c", d.teleportConfigPath)
		out, err := cmd.Output()
		if err != nil {
			return false, nil
		}

		return strings.Contains(string(out), d.nodeName), nil
	}

	if err := pollUntil(ctx, timeout, 1*time.Second, probe); err != nil {
		return fmt.Errorf("docker node failed to join cluster: %w", err)
	}

	d.log.Info("docker SSH node is ready", "name", d.nodeName)

	return nil
}

func (d *dockerNode) saveLogs(ctx context.Context) {
	if d.ctr == nil {
		return
	}

	logPath := d.logFilePath

	logs, err := d.ctr.Logs(ctx)
	if err != nil {
		d.log.Warn("could not get docker node logs", "error", err)
		return
	}
	defer logs.Close()

	f, err := os.Create(logPath)
	if err != nil {
		d.log.Warn("could not create docker node log file", "error", err)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, logs); err != nil {
		d.log.Warn("could not write docker node logs", "error", err)
		return
	}

	d.log.Info("saved docker node logs", "path", logPath)
}

func (d *dockerNode) stop(ctx context.Context) {
	if d.ctr == nil {
		return
	}

	d.log.Info("stopping docker SSH node")

	d.saveLogs(ctx)
	_ = d.ctr.Terminate(ctx, container.TerminateTimeout(10*time.Second))
}

func pullImage(ctx context.Context, image string) error {
	slog.Info("pulling docker image", "image", image)

	cli, err := client.New(client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("creating docker client: %w", err)
	}
	defer cli.Close()

	rc, err := cli.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}
	defer rc.Close()

	_, err = io.Copy(io.Discard, rc)
	return err
}
