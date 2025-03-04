//go:build linux

/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package workloadattest

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// SystemdAttestor attests the identity of a Systemd service.
type SystemdAttestor struct {
	cfg    SystemdAttestorConfig
	log    *slog.Logger
	dialer func(context.Context) (dbusConn, error)
}

func NewSystemdAttestor(cfg SystemdAttestorConfig, log *slog.Logger) *SystemdAttestor {
	return &SystemdAttestor{
		dialer: func(ctx context.Context) (dbusConn, error) {
			return dbus.NewWithContext(ctx)
		},
	}
}

func (a *SystemdAttestor) Attest(ctx context.Context, pid int) (*workloadidentityv1pb.WorkloadAttrsSystemd, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, err := a.dialer(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "creating dbus connection")
	}
	defer conn.Close()

	unit, err := conn.GetUnitNameByPID(ctx, uint32(pid))
	if err != nil {
		return nil, trace.Wrap(err, "getting unit name")
	}

	service, isService := strings.CutSuffix(unit, ".service")
	if !isService {
		return nil, trace.Errorf("unit %q is not a service", unit)
	}

	return &workloadidentityv1pb.WorkloadAttrsSystemd{
		Attested: true,
		Service:  service,
	}, nil
}

type dbusConn interface {
	GetUnitNameByPID(context.Context, uint32) (string, error)
	Close()
}

var _ dbusConn = (*dbus.Conn)(nil)
