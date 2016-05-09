/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bufio"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

var osxUserShellRegexp = regexp.MustCompile("UserShell: (/[^ ]+)\n")

// GetLoginShell determines the login shell for a given username
func GetLoginShell(username string) (string, error) {
	// func to determine user shell on OSX:
	forMac := func() (string, error) {
		dir := "Local/Default/Users/" + username
		out, err := exec.Command("dscl", "localhost", "-read", dir, "UserShell").Output()
		if err != nil {
			log.Warn(err)
			return "", trace.Errorf("cannot determine shell for %s", username)
		}
		m := osxUserShellRegexp.FindStringSubmatch(string(out))
		shell := m[1]
		if shell == "" {
			return "", trace.Errorf("dscl output parsing error getting shell for %s", username)
		}
		return shell, nil
	}
	// func to determine user shell on other unixes (linux)
	forUnix := func() (string, error) {
		f, err := os.Open("/etc/passwd")
		if err != nil {
			return "", trace.Wrap(err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			parts := strings.Split(strings.TrimSpace(scanner.Text()), ":")
			if parts[0] == username && len(parts) > 5 {
				return parts[6], nil
			}
		}
		if scanner.Err() != nil {
			log.Error(scanner.Err())
		}
		return "", trace.Errorf("cannot determine shell for %s", username)
	}
	if runtime.GOOS == "darwin" {
		return forMac()
	}
	return forUnix()
}
