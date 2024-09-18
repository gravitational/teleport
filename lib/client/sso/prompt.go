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

package sso

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
)

func (rd *Redirector) PromptRedirect(redirectURL string) {
	clickableURL := rd.ClickableURL(redirectURL)

	// If a command was found to launch the browser, create and start it.
	if err := OpenURLInBrowser(rd.Browser, clickableURL); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open a browser window for login: %v\n", err)
	}

	// Print the URL to the screen, in case the command that launches the browser did not run.
	// If Browser is set to the special string teleport.BrowserNone, no browser will be opened.
	if rd.Browser == teleport.BrowserNone {
		fmt.Fprintf(os.Stderr, "Use the following URL to authenticate:\n %v\n", clickableURL)
	} else {
		fmt.Fprintf(os.Stderr, "If browser window does not open automatically, open it by ")
		fmt.Fprintf(os.Stderr, "clicking on the link:\n %v\n", clickableURL)
	}
}

// OpenURLInBrowser opens a URL in a web browser.
func OpenURLInBrowser(browser string, URL string) error {
	var execCmd *exec.Cmd
	if browser != teleport.BrowserNone {
		switch runtime.GOOS {
		// macOS.
		case constants.DarwinOS:
			path, err := exec.LookPath(teleport.OpenBrowserDarwin)
			if err == nil {
				execCmd = exec.Command(path, URL)
			}
		// Windows.
		case constants.WindowsOS:
			path, err := exec.LookPath(teleport.OpenBrowserWindows)
			if err == nil {
				execCmd = exec.Command(path, "url.dll,FileProtocolHandler", URL)
			}
		// Linux or any other operating system.
		default:
			path, err := exec.LookPath(teleport.OpenBrowserLinux)
			if err == nil {
				execCmd = exec.Command(path, URL)
			}
		}
	}
	if execCmd != nil {
		if err := execCmd.Start(); err != nil {
			return err
		}
	}

	return nil
}
