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
