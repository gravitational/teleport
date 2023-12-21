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

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func checkDrone() error {
	if _, err := exec.LookPath("drone"); err != nil {
		return fmt.Errorf("can't find drone in $PATH: %w; get it from https://docs.drone.io/cli/install", err)
	}
	if os.Getenv("DRONE_SERVER") == "" || os.Getenv("DRONE_TOKEN") == "" {
		return fmt.Errorf("$DRONE_SERVER and/or $DRONE_TOKEN env vars not set; get them at https://drone.platform.teleport.sh/account")
	}
	return nil
}

func signDroneConfig() error {
	out, err := exec.Command("drone", "sign", "gravitational/teleport", "--save").CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			err = fmt.Errorf("drone signing failed: %v\noutput:\n%s", err, out)
		}
		return err
	}
	return nil
}
