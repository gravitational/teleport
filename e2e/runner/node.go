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
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const nodeImage = "debian:bookworm-slim"

// nodeStartCommand is how the node is launched inside the container.
const nodeStartCommand = "teleport start --insecure -c /etc/teleport/node.yaml"

// mountTracefs mounts tracefs for the tracepoint and kprobe programs to attach through, since Docker
// does not mount it even for a privileged container. Either mount point will do, so failures are left
// for the node to report.
const mountTracefs = "mount -t tracefs nodev /sys/kernel/tracing 2>/dev/null || " +
	"mount -t debugfs nodev /sys/kernel/debug 2>/dev/null || true"

type dockerNode struct {
	log                *slog.Logger
	sshPort            int
	tctlBin            string
	teleportConfigPath string
	logFilePath        string

	imageName     string
	containerName string
	configPath    string
	teleportBin   string

	arch              nodeArch
	enhancedRecording bool

	ctr *container.Container
}

func (d *dockerNode) start(ctx context.Context) error {
	return d.runContainer(ctx)
}

func (d *dockerNode) entrypoint() []string {
	if !d.enhancedRecording {
		return strings.Fields(nodeStartCommand)
	}

	return []string{"sh", "-c", mountTracefs + "; exec " + nodeStartCommand}
}

func (d *dockerNode) removeStale(ctx context.Context) {
	cli, err := dockerAPI()
	if err != nil {
		return
	}

	_, _ = cli.ContainerRemove(ctx, d.containerName, client.ContainerRemoveOptions{Force: true})
}

func (d *dockerNode) runContainer(ctx context.Context) error {
	d.log.Info("starting docker SSH node")

	d.removeStale(ctx)

	sdk, err := dockerSDK()
	if err != nil {
		return err
	}

	ctr, err := container.Run(ctx,
		container.WithClient(sdk),
		container.WithImage(d.imageName),
		container.WithImagePlatform("linux/"+string(d.arch)),
		container.WithPullHandler(func(r io.ReadCloser) error {
			_, err := io.Copy(io.Discard, r)
			return err
		}),
		container.WithName(d.containerName),
		container.WithEntrypoint(d.entrypoint()...),
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

			if d.enhancedRecording {
				hc.Privileged = true
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

		return strings.Contains(string(out), "docker-node"), nil
	}

	if err := pollUntil(ctx, timeout, 1*time.Second, probe); err != nil {
		return fmt.Errorf("docker node failed to join cluster: %w", err)
	}

	d.log.Info("docker SSH node is ready")

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

func pullImage(ctx context.Context, image string, arch nodeArch) error {
	slog.Info("pulling docker image", "image", image, "arch", arch)

	cli, err := dockerAPI()
	if err != nil {
		return err
	}

	rc, err := cli.ImagePull(ctx, image, client.ImagePullOptions{
		Platforms: []ocispec.Platform{{OS: "linux", Architecture: string(arch)}},
	})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}
	defer rc.Close()

	// Wait surfaces failures reported inside the progress stream, which a plain copy would discard.
	// A cached image is still usable when the registry refuses us, so that is only a warning.
	if err := rc.Wait(ctx); err != nil {
		if _, inspectErr := cli.ImageInspect(ctx, image); inspectErr != nil {
			return fmt.Errorf("pulling image: %w", err)
		}

		slog.Warn("could not refresh docker image, using the cached one", "image", image, "error", err)
	}

	return nil
}
