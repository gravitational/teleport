/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"io"
	"os"
	"runtime"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/automaticupgrades/distro"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	// WindowsOS specifies a Windows operating system target
	WindowsOS = "windows"

	// LinuxOS specifies a Linux operating system target
	LinuxOS = "linux"

	// DarwinOS specifies a Darwin operatin system target
	DarwinOS = "darwin"

	// TeleportOSSPackage specifies the teleport oss package name
	TeleportOSSPackage = "teleport"

	// TeleportEnterprisePackage specifies the teleport enterprise package name
	TeleportEnterprisePackage = "teleport-ent"

	// CloudReleaseChannel specifies the stable/cloud release channel
	CloudReleaseChannel = "stable/cloud"

	// RollingReleaseChannel specifies the stable/rolling release channel
	RollingReleaseChannel = "stable/rolling"

	// TeleportAptSourcesList specifies the teleport apt sources list file path
	TeleportAptSourcesList = "/etc/apt/sources.list.d/teleport.list"

	// TeleportAptRepositoryURL specifies the Teleport apt repository url
	TeleportAptRepositoryURL = "https://apt.releases.teleport.dev/ubuntu"
)

// Updater is a client tools updater
type Updater struct {
	// UpdaterConfig specifies the updater configuration
	UpdaterConfig
}

// UpdaterConfig specifies CLI Updater config
type UpdaterConfig struct {
	// Log specifies the logger
	Log logrus.FieldLogger
	// Stdout specifies where to write stdout from command execution
	Stdout io.Writer
	// Stdout specifies where to write stderr from command execution
	Stderr io.Writer

	// TeleportVersion specifies the desired Teleport version to update to
	TeleportVersion string
	// ReleaseChannel specifies the Teleport repository channel. Defaults to
	// CloudReleaseChannel for Teleport Enterprise, and defaults to
	// RollingReleaseChannel for Teleport OSS
	ReleaseChannel string
}

// CheckAndSetDefaults checks and sets default values
func (cfg *UpdaterConfig) CheckAndSetDefaults() error {
	if cfg.TeleportVersion == "" {
		return trace.BadParameter("require TeleportVersion")
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stdout = os.Stderr
	}
	if cfg.Log == nil {
		cfg.Log = utils.NewLogger().WithField(trace.Component, teleport.ComponentCLIUpdater)
	}
	return nil
}

// NewUpdater returns a new client tools updater
func NewUpdater(cfg UpdaterConfig) (*Updater, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Updater{
		UpdaterConfig: cfg,
	}, nil
}

// Update performs the update
func (updater *Updater) Update(ctx context.Context) error {
	switch runtime.GOOS {
	case LinuxOS:
		switch distro.Distribution() {
		case distro.DebianDistro:
			return updater.updateDebian(ctx)
		default:
			return trace.NotImplemented("update is not yet supported on Linux distro %q", distro.Distribution())
		}
	default:
		return trace.NotImplemented("update is not yet supported on %q", runtime.GOOS)
	}
}

// releaseChannel returns the release channel
func (updater *Updater) releaseChannel(teleportPackage string) string {
	if updater.ReleaseChannel != "" {
		return updater.ReleaseChannel
	}

	switch teleportPackage {
	case TeleportOSSPackage:
		return RollingReleaseChannel
	case TeleportEnterprisePackage:
		return CloudReleaseChannel
	default:
		return ""
	}
}
