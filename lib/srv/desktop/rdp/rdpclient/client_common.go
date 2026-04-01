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
	"crypto/md5"
	"encoding/binary"
	"image/png"
	"log/slog"

	"github.com/google/uuid"
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
	// This field is not used when AD is false.
	KDCAddr string

	// AD indicates whether the desktop is part of an Active Directory domain.
	AD bool
}

//nolint:unused // used in client.go that is behind desktop_access_rdp build flag
func (c *Config) checkAndSetDefaults() error {
	if c.Addr == "" {
		return trace.BadParameter("missing Addr in rdpclient.Config")
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

type rdpClientID [16]byte

type uint32Like interface {
	~uint32
}

//nolint:unused // only used in client.go which is ignored by linter
func rdpClientIDToUint32Array[T uint32Like](h rdpClientID) [4]T {
	cArray := [4]T{}
	for idx := range cArray {
		cArray[idx] = T(binary.LittleEndian.Uint32(h[idx*4:]))
	}
	return cArray
}

func newRDPClientID(id string) rdpClientID {
	// In previous revisions of this code, we incorrectly assumed
	// that rdpClientID would always be a UUID. This is not always the case,
	// however, we should continue to attempt to parse rdpClientID as a UUID
	// so that the client ID stays the same for existing users as this
	// may affect RDP licensing.
	// See https://github.com/gravitational/teleport/issues/63766
	parsedUUID, err := uuid.Parse(id)
	if err == nil {
		return rdpClientID(parsedUUID)
	}

	// Fall back to taking a hash of the rdpClientID
	return md5.Sum([]byte(id))
}
