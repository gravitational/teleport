//go:build sessionhelper_embed

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

package reexec

import (
	_ "embed"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
)

var embeddedReexecMemfd atomic.Pointer[os.File]

var embeddedReexecOnce sync.Once
var embeddedReexecError error

func initEmbeddedReexec() (bool, error) {
	embeddedReexecOnce.Do(func() {
		if os.Getenv("TELEPORT_UNSTABLE_DISABLE_EMBEDDED_REEXEC") == "yes" {
			embeddedReexecError = trace.Errorf("embedded binary disabled by TELEPORT_UNSTABLE_DISABLE_EMBEDDED_REEXEC")
			return
		}

		// this name is only displayed as a link in /proc/<pid>/exe or /proc/<pid>/fd/<n>
		mf, err := loadEmbeddedReexec("teleport-sessionhelper", sessionHelperGZ)
		if err != nil {
			embeddedReexecError = trace.Wrap(err)
			return
		}

		embeddedReexecMemfd.Store(mf)
	})

	return true, embeddedReexecError
}

func setLinuxReexecPath(cmd *exec.Cmd) {
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Credential != nil {
		// this should never happen because the main process always launches the
		// "exec" or "networking" subcommands as itself and there's no other
		// direct launches with unprivileged credentials anymore, but if we
		// accidentally launched the session helper directly as an unprivileged
		// child it just wouldn't work due to permissions, because the child
		// wouldn't be able to launch "/proc/<pid of parent>/fd/<n>" and there's
		// no other way to pass a file descriptor such that it's available in
		// the post-fork pre-exec environment but not inherited by the child in
		// go
		cmd.Path = "/proc/self/exe"
		return
	}
	if mf := embeddedReexecMemfd.Load(); mf != nil {
		cmd.Path = mf.Name()
		return
	}
	// we failed to initialize the reexec memfd or we haven't bothered to call
	// [initEmbeddedReexec] in this process (maybe because we're reexecuting a
	// subcommand from a subcommand in the main binary)
	cmd.Path = "/proc/self/exe"
}
