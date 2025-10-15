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

package helpers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/cryptosuites/cryptosuitestest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/common"
)

// TestMainImplementation will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMainImplementation(m *testing.M) {
	logtest.InitLogger(testing.Verbose)

	ctx, cancel := context.WithCancel(context.Background())
	cryptosuitestest.PrecomputeRSAKeys(ctx)
	SetTestTimeouts(3 * time.Second)
	modules.SetInsecureTestMode(true)
	// If the test is re-executing itself, execute the command that comes over
	// the pipe.
	if srv.IsReexec() {
		defer cancel()
		common.Run(common.Options{Args: os.Args[1:]})
		return
	}

	// Otherwise run tests as normal.
	exitCode := m.Run()
	cancel()
	os.Exit(exitCode)
}
