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
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/go-sdk/container"
	"github.com/docker/go-sdk/image"
	apicontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

type dockerNode struct {
	config         *e2eConfig
	imageName      string
	containerName  string
	configPath     string
	dockerfilePath string

	ctr *container.Container
}

func (d *dockerNode) start(ctx context.Context) error {
	if err := d.buildImage(ctx); err != nil {
		return err
	}

	return d.runContainer(ctx)
}

func (d *dockerNode) buildImage(ctx context.Context) error {
	slog.Info("building docker SSH node", "version", d.config.nodeTeleportVersion)

	if _, err := image.BuildFromDir(ctx, filepath.Dir(d.dockerfilePath), filepath.Base(d.dockerfilePath), d.imageName,
		image.WithBuildOptions(client.ImageBuildOptions{
			BuildArgs: map[string]*string{"TELEPORT_VERSION": &d.config.nodeTeleportVersion},
		}),
	); err != nil {
		return fmt.Errorf("building docker image: %w", err)
	}

	return nil
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
	slog.Info("starting docker SSH node")

	d.removeStale(ctx)

	ctr, err := container.Run(ctx,
		container.WithImage(d.imageName),
		container.WithName(d.containerName),
		container.WithExposedPorts(fmt.Sprintf("%d/tcp", d.config.sshPort)),
		container.WithFiles(container.File{
			HostPath:      d.configPath,
			ContainerPath: "/etc/teleport/node.yaml",
			Mode:          0o644,
		}),
		container.WithHostConfigModifier(func(hc *apicontainer.HostConfig) {
			hc.ExtraHosts = []string{"host.docker.internal:host-gateway"}
			hc.PortBindings = network.PortMap{
				network.MustParsePort(fmt.Sprintf("%d/tcp", d.config.sshPort)): []network.PortBinding{
					{HostPort: fmt.Sprintf("%d", d.config.sshPort)},
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
	slog.Debug("waiting for docker node to join cluster")

	probe := func(ctx context.Context) (bool, error) {
		cmd := exec.CommandContext(ctx, d.config.tctlBin, "nodes", "ls",
			"-c", d.config.teleportConfigPath)
		out, err := cmd.Output()
		if err != nil {
			return false, nil
		}

		return strings.Contains(string(out), "docker-node"), nil
	}

	if err := pollUntil(ctx, timeout, 1*time.Second, probe); err != nil {
		return fmt.Errorf("docker node failed to join cluster: %w", err)
	}

	slog.Info("docker SSH node is ready")

	return nil
}

func (d *dockerNode) stop(ctx context.Context) {
	if d.ctr == nil {
		return
	}

	slog.Info("stopping docker SSH node")

	_ = d.ctr.Terminate(ctx, container.TerminateTimeout(10*time.Second))
}
