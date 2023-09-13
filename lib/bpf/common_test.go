/*
Copyright 2019 Gravitational, Inc.

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

package bpf

import (
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// reexecInCGroupCmd is a cmd used to re-exec the test binary and call arbitrary program.
	reexecInCGroupCmd = "reexecCgroup"
	// networkInCgroupCmd is a cmd used to re-exec the test binary and make HTTP call.
	networkInCgroupCmd = "networkCgroup"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()

	// Check if the re-exec was requested.
	if len(os.Args) == 3 {
		var err error

		switch os.Args[1] {
		case reexecInCGroupCmd:
			// Get the command to run passed as the 3rd argument.
			cmd := os.Args[2]

			err = waitAndRun(cmd)
		case networkInCgroupCmd:
			// Get the endpoint to call.
			endpoint := os.Args[2]

			err = callEndpoint(endpoint)
		default:
			os.Exit(2)
		}

		if err != nil {
			// Something went wrong, exit with error.
			os.Exit(1)
		}

		// The rexec was handled and nothing bad happened.
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// TestCheckAndSetDefaults makes sure defaults are set when the user does not
// provide values for the page sizes and hard coded values (like zero or a
// specific page size) are respected when given.
func TestBPFConfig_CheckAndSetDefaults(t *testing.T) {
	perfBufferPageCount := defaults.PerfBufferPageCount
	openPerfBufferPageCount := defaults.OpenPerfBufferPageCount
	zeroPageCount := 0
	udpSilencePeriod := defaults.UDPSilencePeriod
	udpSilenceBufferSize := defaults.UDPSilenceBufferSize
	customUDPSilencePeriod := 1 * time.Minute
	customUDPSilenceBufferSize := 42

	var tests = []struct {
		name string
		got  *servicecfg.BPFConfig
		want *servicecfg.BPFConfig
	}{
		{
			name: "all defaults",
			got:  &servicecfg.BPFConfig{},
			want: &servicecfg.BPFConfig{
				CommandBufferSize:    &perfBufferPageCount,
				DiskBufferSize:       &openPerfBufferPageCount,
				NetworkBufferSize:    &perfBufferPageCount,
				CgroupPath:           defaults.CgroupPath,
				UDPSilencePeriod:     &udpSilencePeriod,
				UDPSilenceBufferSize: &udpSilenceBufferSize,
			},
		},
		{
			name: "values set",
			got: &servicecfg.BPFConfig{
				CommandBufferSize:    &zeroPageCount,
				DiskBufferSize:       &zeroPageCount,
				NetworkBufferSize:    &perfBufferPageCount,
				CgroupPath:           "/my/cgroup/",
				UDPSilencePeriod:     &customUDPSilencePeriod,
				UDPSilenceBufferSize: &customUDPSilenceBufferSize,
			},
			want: &servicecfg.BPFConfig{
				CommandBufferSize:    &zeroPageCount,
				DiskBufferSize:       &zeroPageCount,
				NetworkBufferSize:    &perfBufferPageCount,
				CgroupPath:           "/my/cgroup/",
				UDPSilencePeriod:     &customUDPSilencePeriod,
				UDPSilenceBufferSize: &customUDPSilenceBufferSize,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.got.CheckAndSetDefaults()
			require.NoError(t, err, "CheckAndSetDefaults errored")

			if diff := cmp.Diff(test.want, test.got); diff != "" {
				t.Errorf("CheckAndSetDefaults mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

// waitAndRun wait for continue signal to be generated an executes the
// passed command and waits until returns.
func waitAndRun(cmd string) error {
	if err := waitForContinue(); err != nil {
		return err
	}

	return osexec.Command(cmd).Run()
}

// callEndpoint wait for continue signal to be generated an executes HTTP GET
// on provided endpoint.
func callEndpoint(endpoint string) error {
	if err := waitForContinue(); err != nil {
		return err
	}

	resp, err := http.Get(endpoint)
	if resp != nil {
		// Close the body to make our linter happy.
		_ = resp.Body.Close()
	}

	return err
}

// waitForContinue opens FD 3 and waits the signal from parent process that
// the cgroup is being observed and the even can be generated.
func waitForContinue() error {
	waitFD := os.NewFile(3, "/proc/self/fd/3")
	defer waitFD.Close()

	buff := make([]byte, 1)
	if _, err := waitFD.Read(buff); err != nil && err != io.EOF {
		return err
	}

	return nil
}
