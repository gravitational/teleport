// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/gravitational/trace"
)

// polkitAllowUserInteraction allows polkit to prompt the user
// for a password if it is required.
const polkitAllowUserInteraction = uint32(1)

// introspectNode describes the exported D-Bus API. Update it if any method
// signature is changed or new methods are added.
var introspectNode = &introspect.Node{
	Name: vnetDBusObjectPath,
	Interfaces: []introspect.Interface{
		introspect.IntrospectData,
		{
			Name: vnetDBusInterface,
			Methods: []introspect.Method{
				{
					Name: "Start",
					Args: []introspect.Arg{
						{Name: "addr", Type: "s", Direction: "in"},
						{Name: "credPath", Type: "s", Direction: "in"},
					},
				},
				{Name: "Stop"},
			},
		},
	},
}

// RunLinuxDBusService runs the VNet D-Bus service that can start the VNet admin process.
func RunLinuxDBusService(ctx context.Context) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return trace.Wrap(err, "connecting to system D-Bus")
	}
	defer conn.Close()

	serviceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	daemon := &dbusDaemon{
		ctx:    serviceCtx,
		cancel: cancel,
		conn:   conn,
	}
	if err := conn.Export(daemon, dbus.ObjectPath(vnetDBusObjectPath), vnetDBusInterface); err != nil {
		return trace.Wrap(err, "exporting D-Bus object")
	}
	if err := conn.Export(
		introspect.NewIntrospectable(introspectNode),
		dbus.ObjectPath(vnetDBusObjectPath),
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return trace.Wrap(err, "exporting D-Bus introspection")
	}

	reply, err := conn.RequestName(vnetDBusServiceName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return trace.Wrap(err, "requesting D-Bus name")
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return trace.Errorf("D-Bus name %s is already owned", vnetDBusServiceName)
	}
	log.InfoContext(serviceCtx, "Acquired D-Bus name", "name", vnetDBusServiceName)

	<-serviceCtx.Done()
	return nil
}

type dbusDaemon struct {
	ctx    context.Context
	cancel context.CancelFunc
	conn   *dbus.Conn

	mu      sync.Mutex
	started bool
}

// Start is a D-Bus method that starts the VNet admin process.
func (d *dbusDaemon) Start(addr, credPath string, sender dbus.Sender) *dbus.Error {
	if err := d.authorize(sender); err != nil {
		return dbus.MakeFailedError(trace.Wrap(err, "authorization failed"))
	}

	uid, err := d.lookupSenderUID(sender)
	if err != nil {
		return dbus.MakeFailedError(trace.Wrap(err, "looking up D-Bus sender UID"))
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started {
		return dbus.MakeFailedError(trace.Errorf("VNet admin process already started"))
	}
	d.started = true

	log.InfoContext(d.ctx, "Starting VNet admin process", "uid", uid)

	go func() {
		err := RunLinuxAdminProcess(d.ctx, LinuxAdminProcessConfig{
			ClientApplicationServiceAddr: addr,
			ServiceCredentialPath:        credPath,
		})
		// TODO: D-Bus supports signals, we might want to emit a signal when the admin process exits.
		if err != nil {
			log.ErrorContext(d.ctx, "VNet admin process exited with error", "error", err)
		}
		d.cancel()
	}()

	return nil
}

// Stop is a D-Bus method that stops the VNet admin process.
func (d *dbusDaemon) Stop(sender dbus.Sender) *dbus.Error {
	if err := d.authorize(sender); err != nil {
		return dbus.MakeFailedError(trace.Wrap(err, "authorization failed"))
	}
	uid, err := d.lookupSenderUID(sender)
	if err != nil {
		return dbus.MakeFailedError(trace.Wrap(err, "looking up D-Bus sender UID"))
	}
	// We intentionally do not reset started here to avoid a race with Start
	// while the process is exiting. A new Start is allowed only after
	// a new daemon instance is started.
	//
	// D-Bus activation can start the daemon on any method call. We allow
	// Stop before Start so the service can exit immediately instead of idling
	// waiting for a Start call that may never come.
	log.InfoContext(d.ctx, "Stopping VNet admin process", "uid", uid)
	d.cancel()
	return nil
}

func (d *dbusDaemon) authorize(sender dbus.Sender) error {
	uid, err := d.lookupSenderUID(sender)
	if err != nil {
		return trace.Wrap(err, "looking up D-Bus sender UID")
	}
	if uid == 0 {
		return nil
	}

	subject := polkitSubject{
		Kind: "system-bus-name",
		Details: map[string]dbus.Variant{
			"name": dbus.MakeVariant(string(sender)),
		},
	}
	var result struct {
		Authorized bool
		Challenge  bool
		Details    map[string]string
	}
	if err := d.conn.Object("org.freedesktop.PolicyKit1", "/org/freedesktop/PolicyKit1/Authority").
		Call(
			"org.freedesktop.PolicyKit1.Authority.CheckAuthorization",
			0,
			subject,
			vnetPolkitAction,
			map[string]string{},
			polkitAllowUserInteraction,
			"",
		).Store(&result); err != nil {
		return trace.Wrap(err, "checking polkit authorization")
	}
	if !result.Authorized {
		if result.Challenge {
			return trace.Errorf("polkit authentication required")
		}
		return trace.Errorf("polkit authorization denied")
	}
	return nil
}

func (d *dbusDaemon) lookupSenderUID(sender dbus.Sender) (uint32, error) {
	var uid uint32
	if err := d.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").
		Call("org.freedesktop.DBus.GetConnectionUnixUser", 0, string(sender)).
		Store(&uid); err != nil {
		return 0, trace.Wrap(err, "querying D-Bus sender UID")
	}
	return uid, nil
}

type polkitSubject struct {
	Kind    string
	Details map[string]dbus.Variant
}
