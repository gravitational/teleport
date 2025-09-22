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

package memory

import (
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestMemory(t *testing.T) {
	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		cfg, err := test.ApplyOptions(options)

		if err != nil {
			return nil, nil, err
		}

		if cfg.ConcurrentBackend != nil {
			if _, ok := cfg.ConcurrentBackend.(*Memory); !ok {
				return nil, nil, trace.BadParameter("target is not a Memory backend")
			}
			return cfg.ConcurrentBackend, nil, nil
		}

		clock := clockwork.NewFakeClock()
		mem, err := New(Config{
			Clock:  clock,
			Mirror: cfg.MirrorMode,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return mem, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}
