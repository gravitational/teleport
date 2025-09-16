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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/utils"
)

func TestSystemdAttestor_Success(t *testing.T) {
	attestor := NewSystemdAttestor(
		SystemdAttestorConfig{
			Enabled: true,
		},
		utils.NewSlogLoggerForTests(),
	)

	attestor.dbusDialer = func(context.Context) (dbusConn, error) {
		return testDbusConn{unit: "foo.service"}, nil
	}

	attrs, err := attestor.Attest(context.Background(), 1)
	require.NoError(t, err)

	expected := &workloadidentityv1.WorkloadAttrsSystemd{
		Attested: true,
		Service:  "foo",
	}
	require.Empty(t, cmp.Diff(expected, attrs, protocmp.Transform()))
}

func TestSystemdAttestor_NonService(t *testing.T) {
	attestor := NewSystemdAttestor(
		SystemdAttestorConfig{
			Enabled: true,
		},
		utils.NewSlogLoggerForTests(),
	)

	attestor.dbusDialer = func(context.Context) (dbusConn, error) {
		return testDbusConn{unit: "user.scope"}, nil
	}

	_, err := attestor.Attest(context.Background(), 1)
	require.ErrorContains(t, err, "not a service")
}

type testDbusConn struct {
	unit string
	err  error
}

func (testDbusConn) Close() {}

func (t testDbusConn) GetUnitNameByPID(context.Context, uint32) (string, error) {
	return t.unit, t.err
}
