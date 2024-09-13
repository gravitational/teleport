//go:build darwin

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func replace(path string, hash string) error {
	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}

	// Use "pkgutil" from the filesystem to expand the archive. In theory .pkg
	// files are xz archives, however it's still safer to use "pkgutil" in-case
	// Apple makes non-standard changes to the format.
	//
	// Full command: pkgutil --expand-full NAME.pkg DIRECTORY/
	pkgutil, err := exec.LookPath("pkgutil")
	if err != nil {
		return trace.Wrap(err)
	}
	expandPath := filepath.Join(dir, hash+"-pkg")
	out, err := exec.Command(pkgutil, "--expand-full", path, expandPath).Output()
	if err != nil {
		log.Debugf("Failed to run pkgutil: %v.", out)
		return trace.Wrap(err)
	}

	for _, app := range []string{"tsh", "tctl"} {
		// The first time a signed and notarized binary macOS application is run,
		// execution is paused while it gets sent to Apple to verify. Once Apple
		// approves the binary, the "com.apple.macl" extended attribute is added
		// and the process is allow to execute. This process is not concurrent, any
		// other operations (like moving the application) on the application during
		// this time will lead to the application being sent SIGKILL.
		//
		// Since {tsh, tctl} have to be concurrent, execute {tsh, tctl} before
		// performing any swap operations. This ensures that the "com.apple.macl"
		// extended attribute is set and macOS will not send a SIGKILL to the
		// process if multiple processes are trying to operate on it.
		expandExecPath := filepath.Join(expandPath, "Payload", app+".app", "Contents", "MacOS", app)
		command := exec.Command(expandExecPath, "version", "--client")
		command.Env = []string{teleportToolsVersion + "=off"}
		if err := command.Run(); err != nil {
			return trace.Wrap(err)
		}

		// Due to macOS applications not being a single binary (they are a
		// directory), atomic operations are not possible. To work around this, use
		// a symlink (which can be atomically swapped), then do a cleanup pass
		// removing any stale copies of the expanded package.
		oldName := filepath.Join(expandPath, "Payload", app+".app", "Contents", "MacOS", app)
		newName := filepath.Join(dir, app)
		if err := renameio.Symlink(oldName, newName); err != nil {
			return trace.Wrap(err)
		}
	}

	// Perform a cleanup pass to remove any old copies of "{tsh, tctl}.app".
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if hash+"-pkg" == info.Name() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), "-pkg") {
			return nil
		}

		// Found a stale expanded package.
		if err := os.RemoveAll(filepath.Join(dir, info.Name())); err != nil {
			return err
		}

		return nil
	})

	return nil
}
