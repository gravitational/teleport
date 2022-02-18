// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func checkTDR() error {
	if _, err := exec.LookPath("tdr"); err != nil {
		return fmt.Errorf("can't find tdr in $PATH: %w; get it from https://github.com/gravitational/tdr/", err)
	}
	if os.Getenv("DRONE_SERVER") == "" || os.Getenv("DRONE_TOKEN") == "" {
		return fmt.Errorf("$DRONE_SERVER and/or $DRONE_TOKEN env vars not set; get them at https://drone.teleport.dev/account")
	}
	return nil
}

func signDroneConfig() error {
	out, err := exec.Command("tdr", "sign", "gravitational/teleport", "--save").CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			err = fmt.Errorf("drone signing failed: %v\noutput:\n%s", err, out)
		}
		return err
	}
	return nil
}
