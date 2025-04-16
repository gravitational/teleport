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

// Package rdpclient implements an RDP client.
package rdpclient

import (
	"context"
	"image/png"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// LicenseStore implements client-side license storage for Microsoft
// Remote Desktop Services (RDS) licenses.
type LicenseStore interface {
	WriteRDPLicense(ctx context.Context, key *types.RDPLicenseKey, license []byte) error
	ReadRDPLicense(ctx context.Context, key *types.RDPLicenseKey) ([]byte, error)
}

// Config for creating a new Client.
type Config struct {
	// Addr is the network address of the RDP server, in the form host:port.
	Addr string

	LicenseStore LicenseStore
	HostID       string

	// UserCertGenerator generates user certificates for RDP authentication.
	GenerateUserCert GenerateUserCertFn
	CertTTL          time.Duration

	// AuthorizeFn is called to authorize a user connecting to a Windows desktop.
	AuthorizeFn func(login string) error

	// Conn handles TDP messages between Windows Desktop Service
	// and a Teleport Proxy.
	Conn *tdp.Conn

	// Encoder is an optional override for PNG encoding.
	Encoder *png.Encoder

	// AllowClipboard indicates whether the RDP connection should enable
	// clipboard sharing.
	AllowClipboard bool

	// AllowDirectorySharing indicates whether the RDP connection should enable
	// directory sharing.
	AllowDirectorySharing bool

	// ShowDesktopWallpaper determines whether desktop sessions will show a
	// user-selected wallpaper vs a system-default, single-color wallpaper.
	ShowDesktopWallpaper bool

	// NLA indicates whether the client should perform Network Level Authentication
	// (NLA) when initiating the RDP session.
	NLA bool

	// Width and Height optionally override the dimensions received from
	// the browser and force the session to use a particular size.
	Width, Height uint32

	// Logger is the logger for status messages.
	Logger *slog.Logger

	// ComputerName is the name used to communicate with KDC.
	// Used for NLA support when AD is true.
	ComputerName string

	// KDCAddr is the address of Key Distribution Center.
	// This is used to support RDP Network Level Authentication (NLA)
	// when connecting to hosts enrolled in Active Directory.
	// This filed is not used when AD is false.
	KDCAddr string

	// AD indicates whether the desktop is part of an Active Directory domain.
	AD bool
}

// GenerateUserCertFn generates user certificates for RDP authentication.
type GenerateUserCertFn func(ctx context.Context, username string, ttl time.Duration) (certDER, keyDER []byte, err error)

//nolint:unused // used in client.go that is behind desktop_access_rdp build flag
func (c *Config) checkAndSetDefaults() error {
	if c.Addr == "" {
		return trace.BadParameter("missing Addr in rdpclient.Config")
	}
	if c.GenerateUserCert == nil {
		return trace.BadParameter("missing GenerateUserCert in rdpclient.Config")
	}
	if c.Conn == nil {
		return trace.BadParameter("missing Conn in rdpclient.Config")
	}
	if c.AuthorizeFn == nil {
		return trace.BadParameter("missing AuthorizeFn in rdpclient.Config")
	}
	if c.Logger == nil {
		return trace.BadParameter("missing Logger in rdpclient.Config")
	}
	if c.Encoder == nil {
		c.Encoder = tdp.PNGEncoder()
	}
	c.Logger = c.Logger.With("rdp_addr", c.Addr)
	return nil
}

// hasSizeOverride returns true if the width and height have been set.
// This will be true when a user has specified a fixed `screen_size` for
// a given desktop.
func (c *Config) hasSizeOverride() bool { //nolint:unused // used in client.go that is behind desktop_access_rdp build flag
	return c.Width != 0 && c.Height != 0
}
