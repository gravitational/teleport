//go:build linux
// +build linux

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport"
)

// memFile creates a file in memory and returns a file descriptor.
func memFile(name string, fileContent []byte) (int, error) {
	// Create the in-memory file descriptor
	fd, err := unix.MemfdCreate(name, 0)
	if err != nil {
		return 0, trace.Wrap(err, "memfd create")
	}
	// Set the size of the file to the size of the byte slice
	err = unix.Ftruncate(fd, int64(len(fileContent)))
	if err != nil {
		return 0, trace.Wrap(err, "ftruncate")
	}

	// Map the file into memory
	data, err := unix.Mmap(fd, 0, len(fileContent), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return 0, trace.Wrap(err, "mmap memory")
	}

	// Write content to the in-memory file
	copy(data, fileContent)

	// Unmap the file from memory
	err = unix.Munmap(data)
	if err != nil {
		return 0, trace.Wrap(err, "munmap memory")
	}

	return fd, nil
}

func reexecToShell(ctx context.Context, kubeconfigData []byte) (err error) {
	// Create in-memory file containing kubeconfig and return file descriptor.
	fd, err := memFile("proxy-kubeconfig", kubeconfigData)
	if err != nil {
		return trace.Wrap(err, "failed to create in-memory file")
	}

	// Set filepath to our newly created in-memory file descriptor.
	fp := fmt.Sprintf("/proc/self/fd/%d", fd)

	// Open the file
	f := os.NewFile(uintptr(fd), fp)
	defer func() { err = trace.NewAggregate(err, f.Close()) }()

	// Prepare to re-exec shell
	command := "/bin/bash"
	if shell, ok := os.LookupEnv("SHELL"); ok {
		command = shell
	}

	cmd := exec.CommandContext(ctx, command)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	// Set KUBECONFIG in the environment. Even if it was already set, we override it.
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", teleport.EnvKubeConfig, "/proc/self/fd/3"))
	// Pass the file descriptor to the child process as an extra file
	// descriptor. It will be available as fd 3 in "/proc/self/fd/3".
	cmd.ExtraFiles = []*os.File{f}

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	if err := cmd.Wait(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
