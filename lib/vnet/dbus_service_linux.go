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
	"errors"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet/polkit"
)

const polkitAuthorizationTimeout = 30 * time.Second

// introspectNode describes the exported D-Bus API. Update it if any method
// signature is changed or new methods are added.
//
// D-Bus is strict about method signatures. If a client sends a different number
// or type of arguments than the service expects, the call fails with an error.
// There is no built-in forward or backward compatibility for method arguments.
//
// In practice this is unlikely to be a concern because the systemd unit
// typically runs the same tsh binary as the client, so both sides are expected
// to be the same version.
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
						{Name: "socketPath", Type: "s", Direction: "in"},
					},
				},
				{Name: "Stop"},
			},
		},
	},
}

// RunLinuxDBusService runs the privileged VNet D-Bus daemon on the system bus.
// It claims the VNet service name and exports the VNet interface that
// exposes Start and Stop methods that normal client processes can call via
// system D-Bus. The daemon blocks until the context is canceled.
func RunLinuxDBusService(ctx context.Context) error {
	daemon, err := newDBusDaemon()
	if err != nil {
		return trace.Wrap(err)
	}

	stop := context.AfterFunc(ctx, daemon.Close)
	defer stop()

	return trace.Wrap(daemon.Wait())
}

func newDBusDaemon() (_ *dbusDaemon, err error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, trace.Wrap(err, "connecting to system D-Bus")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
			_ = conn.Close()
		}
	}()

	daemon := &dbusDaemon{
		conn: conn,
		done: make(chan error, 1),
		startAdminProcess: func(socketPath string) error {
			return trace.Wrap(RunLinuxAdminProcess(ctx, LinuxAdminProcessConfig{
				ClientApplicationServiceSocketPath: socketPath,
			}))
		},
		cancelAdminProcess: cancel,
	}

	if err := conn.Export(daemon, dbus.ObjectPath(vnetDBusObjectPath), vnetDBusInterface); err != nil {
		return nil, trace.Wrap(err, "exporting D-Bus object")
	}
	if err := conn.Export(
		introspect.NewIntrospectable(introspectNode),
		dbus.ObjectPath(vnetDBusObjectPath),
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return nil, trace.Wrap(err, "exporting D-Bus introspection")
	}

	reply, err := conn.RequestName(vnetDBusServiceName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, trace.Wrap(err, "requesting D-Bus name")
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, trace.Errorf("D-Bus name %s is already owned", vnetDBusServiceName)
	}

	log.InfoContext(context.Background(), "Acquired D-Bus name", "name", vnetDBusServiceName)
	return daemon, nil
}

type dbusDaemon struct {
	mu      sync.Mutex
	conn    *dbus.Conn
	started bool
	closing bool
	done    chan error // buffered 1; receives the admin process error or nil

	startAdminProcess  func(socketPath string) error
	cancelAdminProcess context.CancelFunc
}

func (d *dbusDaemon) Close() {
	d.mu.Lock()
	if d.closing {
		d.mu.Unlock()
		return
	}
	d.closing = true
	started := d.started
	d.mu.Unlock()

	d.cancelAdminProcess()
	_ = d.conn.Close()

	// If no admin process goroutine was started, unblock Wait.
	if !started {
		d.done <- nil
	}
}

func (d *dbusDaemon) Wait() error {
	err := <-d.done
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// Start starts actual VNet admin process with passed unix socket path.
// It uses polkit to authorize the calling D-Bus sender.
// It returns an error if the admin process has already been started.
func (d *dbusDaemon) Start(socketPath string, sender dbus.Sender) *dbus.Error {
	uid, err := d.authorize(sender)
	if err != nil {
		return dbus.MakeFailedError(trace.Wrap(err, "authorization failed"))
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closing {
		return dbus.MakeFailedError(trace.Errorf("VNet D-Bus daemon is shutting down"))
	}
	if d.started {
		return dbus.MakeFailedError(trace.Errorf("VNet admin process already started"))
	}
	d.started = true
	log.InfoContext(context.Background(), "Starting VNet admin process", "uid", uid)

	go func() {
		err := d.startAdminProcess(socketPath)
		// TODO(tangyatsu): D-Bus supports signals, we might want to emit a signal when the admin process exits.
		if err != nil && !errors.Is(err, context.Canceled) {
			log.ErrorContext(context.Background(), "VNet admin process exited with error", "error", err)
		} else {
			log.InfoContext(context.Background(), "VNet admin process exited")
		}
		d.done <- err
		d.Close()
	}()

	return nil
}

// Stop stops actual VNet admin process and exits the daemon.
// It uses polkit to authorize the calling D-Bus sender.
func (d *dbusDaemon) Stop(sender dbus.Sender) *dbus.Error {
	uid, err := d.authorize(sender)
	if err != nil {
		return dbus.MakeFailedError(trace.Wrap(err, "authorization failed"))
	}
	// We intentionally do not reset started here to avoid a race with Start
	// while the process is exiting. A new Start is allowed only after
	// a new daemon instance is started.
	//
	// D-Bus activation can start the daemon on any method call. We allow
	// Stop before Start so the service can exit immediately instead of idling
	// waiting for a Start call that may never come.
	log.InfoContext(context.Background(), "Stopping VNet admin process", "uid", uid)
	d.Close()
	return nil
}

// authorize checks polkit authorization for the calling D-Bus sender and
// returns the sender UID.
func (d *dbusDaemon) authorize(sender dbus.Sender) (uint32, error) {
	uid, err := d.lookupSenderUID(sender)
	if err != nil {
		return 0, trace.Wrap(err, "looking up D-Bus sender UID")
	}
	if uid == 0 {
		// Always allow root to start the daemon.
		return uid, nil
	}

	authCtx, cancel := context.WithTimeout(d.conn.Context(), polkitAuthorizationTimeout)
	defer cancel()

	subject := polkit.NewSystemBusNameSubject(string(sender))
	result, err := polkit.CheckAuthorization(
		authCtx,
		d.conn,
		subject,
		vnetPolkitAction,
		map[string]string{},
		true,
		"",
	)
	if err != nil {
		return 0, err
	}
	if !result.Authorized {
		if result.Challenge {
			return 0, trace.AccessDenied("polkit authentication required")
		}
		return 0, trace.AccessDenied("polkit authorization denied")
	}
	return uid, nil
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
