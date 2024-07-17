//go:build linux

package unit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

const (
	// upgraderPath is the path to the upgrader executable relative to this test file.
	upgraderPath = "../systemd-unit-upgrader/rootfs/usr/sbin/teleport-upgrade"

	// nopInstallPrefix is the prefix used to signify that a nop install attempt happened (i.e. that
	// the upgrader would have attempted a real install if a different installer were configured).
	nopInstallPrefix = "nop-install:"

	// configVar is the variable used to override the default config dir location.
	configVar      = "TELEPORT_UPGRADE_CONFIG"
	agentConfigVar = "TELEPORT_AGENT_CONFIG"
	stateVar       = "TELEPORT_UPGRADE_STATE"
)

// output
type output struct {
	stdout  string
	stderr  string
	success bool
}

// GetNopInstall seeks and parses the nop upgrade line output, extracting target
// name, version and restart mode.
func (o *output) GetNopInstall() (params upgradeParams, ok bool) {
	for _, ln := range strings.Split(o.stdout, "\n") {
		ln = strings.TrimSpace(ln)
		ln, ok := strings.CutPrefix(ln, nopInstallPrefix)
		if !ok {
			continue
		}
		ln = strings.TrimSpace(ln)

		fields := strings.Split(ln, " ")
		if len(fields) != 2 {
			continue
		}

		target, version, ok := strings.Cut(fields[0], "=")
		if !ok {
			continue
		}

		restartMode := fields[1]

		return upgradeParams{
			target:      target,
			version:     version,
			restartMode: restartMode,
		}, true
	}

	return upgradeParams{}, false
}

// upgradeParams represents the output of a 'nop' upgrade attempt.
type upgradeParams struct {
	target      string
	version     string
	restartMode string
}

func runUpgrader(subcommand string, configDir, stateDir string) (output, error) {
	agentConfig := fmt.Sprintf("%s/%s", configDir, "teleport.yaml")
	cmd := exec.Command(upgraderPath, subcommand)
	cmd.Env = []string{
		fmt.Sprintf("%s=%s", configVar, configDir),
		fmt.Sprintf("%s=%s", agentConfigVar, agentConfig),
		fmt.Sprintf("%s=%s", stateVar, stateDir),
	}
	cmd.Stdout = new(strings.Builder)
	cmd.Stderr = new(strings.Builder)

	if err := cmd.Run(); err != nil {
		// ExitError just means non-zero exit code... handled elsewhere.
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return output{}, trace.Wrap(err)
		}
	}

	return output{
		stdout:  cmd.Stdout.(*strings.Builder).String(),
		stderr:  cmd.Stderr.(*strings.Builder).String(),
		success: cmd.ProcessState.Success(),
	}, nil
}

// testCase is a helper for setting up a test-case. running multiple cases against the same
// config dir preserves previous state unless explicitly overwritten by a cfg parameter.
type testCase struct {
	cmd, dir string
	cfg      map[string]string
	exclude  []string
	agentCfg string
}

func (t *testCase) Run() (output, error) {
	if err := t.setTestDefaults(); err != nil {
		return output{}, trace.Wrap(err)
	}

	for param, value := range t.cfg {
		if err := t.set(param, value); err != nil {
			return output{}, trace.Wrap(err)
		}
	}

	for _, param := range t.exclude {
		if err := t.del(param); err != nil {
			return output{}, trace.Wrap(err)
		}
	}

	if t.agentCfg != "" {
		if err := t.set("teleport.yaml", t.agentCfg); err != nil {
			return output{}, trace.Wrap(err)
		}
	}

	cmd := t.cmd
	if cmd == "" {
		cmd = "run"
	}

	return runUpgrader(cmd, t.dir, t.dir)
}

// setTestDefaults sets the config parameters that are consistent for any test case.
func (t *testCase) setTestDefaults() error {
	if err := t.set("insecure", "yes"); err != nil {
		return trace.Wrap(err)
	}

	if err := t.set("debug", "yes"); err != nil {
		return trace.Wrap(err)
	}

	if err := t.set("installer", "nop"); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (t *testCase) set(name string, value string) error {
	err := os.WriteFile(filepath.Join(t.dir, name), []byte(value), 0o644)
	return trace.Wrap(err)
}

func (t *testCase) del(name string) error {
	err := os.Remove(filepath.Join(t.dir, name))
	if os.IsNotExist(err) {
		return nil
	}
	return trace.Wrap(err)
}

func (t *testCase) Get(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(t.dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", trace.Wrap(err)
	}

	return string(b), nil
}

func (t *testCase) Expire(name string) error {
	err := os.Chtimes(filepath.Join(t.dir, name), time.Time{}, time.Now().Add(-time.Hour))
	return trace.Wrap(err)
}

// stripCfgValue strips extra whitespace and comment-like lines from a string.
func stripCfgValue(original string) string {
	var stripped []string
	for _, ln := range strings.Split(original, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "# ") {
			continue
		}
		stripped = append(stripped, ln)
	}

	return strings.Join(stripped, "\n")
}

// currentSchedule builds a reasonable schedule value that places us within a current window,
// as well as having one past and one future window.
func currentSchedule() string {
	now := time.Now().Unix()
	return fmt.Sprintf("# some-comment\n%d %d\n%d %d\n%d %d\n",
		now-120, now-60, // past window
		now-1, now+99, // current window
		now+120, now+180, // future window
	)
}

// TestMissingEndpoint verifies that he upgrader refuses to run without a valid endpoint
// configured.
func TestMissingEndpoint(t *testing.T) {
	// tc1 covers the case of installer file being empty (or containing only comments)
	tc1 := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"endpoint": "",
		},
	}

	out, err := tc1.Run()
	require.NoError(t, err)
	require.False(t, out.success)
	require.Contains(t, out.stderr, "missing required config")

	// tc2 covers the case of endpoint file being completely missing
	tc2 := testCase{
		dir: t.TempDir(),
		exclude: []string{
			"endpoint",
		},
	}

	out, err = tc2.Run()
	require.NoError(t, err)
	require.False(t, out.success)
	require.Contains(t, out.stderr, "missing required config")
}

// TestUpgraderBasics verifies the standard paths to upgrade.
func TestUpgraderBasics(t *testing.T) {
	endpoint := NewUpgradeEndpoint("")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up basic test-case that should cause us to fire off an install attempt
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"endpoint":               fmt.Sprintf("localhost:%s/v1/stable/cloud", port),
			"schedule":               currentSchedule(),
			"state-version-override": "1.2.3", // overrides the upgrader's view of the currently installed teleport version
		},
	}

	out, err := tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
	require.Equal(t, "reload", nop.restartMode)

	tc.cfg["restart-mode"] = "restart"

	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok = out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
	require.Equal(t, "restart", nop.restartMode)

	delete(tc.cfg, "restart-mode")
	tc.exclude = []string{"restart-mode"}

	// simulate a successful upgrade by overriding the upgrader's "current version" view to
	// now equal the version served by the endpoint.
	tc.cfg["state-version-override"] = "2.3.4"

	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// expect that no install happened this time
	_, ok = out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// bump the endpoint version so that upgrade attempts start happening again
	endpoint.SetVersion("3.4.5")

	// refresh state-target-version
	require.NoError(t, tc.Expire("state-target-version"))

	// blank the schedule so that upgrader believes agent may be unhealthy
	tc.cfg["schedule"] = ""

	// the first run w/ unhealthy schedule should result in us setting the unhealthy marker
	// but not in an actual upgrade attempt.
	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	_, ok = out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// check for expected unhealthy state marker
	us, err := tc.Get("state-unhealthy")
	require.NoError(t, err)
	require.Equal(t, "yes", stripCfgValue(us), "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, us)

	// run again, this time we expect the unhealthy marker state to cause an upgrade
	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok = out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "3.4.5", nop.version)
	// upgrades with an unhealthy schedule state are full restarts
	require.Equal(t, "restart", nop.restartMode)

	// unhealthy marker state should be cleared/removed
	us, err = tc.Get("state-unhealthy")
	require.NoError(t, err)
	require.Equal(t, "", us, "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, us)
}

// TestUpgraderCritical verifies the expected behavior of the 'critical' endpoint mode.
func TestUpgraderCritical(t *testing.T) {
	endpoint := NewUpgradeEndpoint("")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up a healthy configuration that is not within an upgrade window
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"endpoint":               fmt.Sprintf("localhost:%s/v1/stable/cloud", port),
			"schedule":               fmt.Sprintf("%d %d", time.Now().Unix()+99, time.Now().Unix()+110),
			"state-version-override": "1.2.3", // overrides the upgrader's view of the currently installed teleport version
		},
	}

	out, err := tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// not within upgrade window, nothing should happen
	_, ok := out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// unhealthy marker should not be set
	us, err := tc.Get("state-unhealthy")
	require.NoError(t, err)
	require.Equal(t, "", us, "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, us)

	// go into 'critical' mode
	endpoint.SetCritical("yes")

	// refresh state-critical
	require.NoError(t, tc.Expire("state-critical"))

	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// install should have happened this time
	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)
	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
	// critical upgrades cause a full restart rather than a reload
	require.Equal(t, "restart", nop.restartMode)

	// revert to non-critical and re-check that we are in a "not upgrading but healthy" state.
	endpoint.SetCritical("no")

	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// not within upgrade window, nothing should happen
	_, ok = out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// unhealthy marker still not set
	us, err = tc.Get("state-unhealthy")
	require.NoError(t, err)
	require.Equal(t, "", us, "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, us)
}

func TestUnknownVersionScenarios(t *testing.T) {
	endpoint := NewUpgradeEndpoint("")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up a configuration that can't discover currently installed teleport
	// version, but is otherwise healthy.
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"endpoint":               fmt.Sprintf("localhost:%s/v1/stable/cloud", port),
			"schedule":               fmt.Sprintf("%d %d", time.Now().Unix()+99, time.Now().Unix()+110),
			"state-version-override": "fail", // set script to be unable to determine current teleport version
		},
	}

	out, err := tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// not within upgrade window, nothing should happen
	_, ok := out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// modify schedule so that we are now within an upgrade window
	tc.cfg["schedule"] = fmt.Sprintf("%d %d", time.Now().Unix()-1, time.Now().Unix()+99)

	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)
	require.Contains(t, out.stderr, "failed to detect current version")

	// expect that we now succeed despite not knowing the current version
	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)
	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
	require.Equal(t, "reload", nop.restartMode)

	// shutdown the version endpoint
	endpoint.Shutdown(context.Background())
	listener.Close()

	out, err = tc.Run()
	require.NoError(t, err)

	// if there is no version endpoint, upgrader cannot function
	require.False(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)
}

const testAgentCfg = `
# By default, this file should be stored in /etc/teleport.yaml

# Configuration file version. The current version is "v3".
version: v3

# This section of the configuration file applies to all teleport
# services.
teleport:
    nodename: graviton
    data_dir: /var/lib/teleport
    auth_token: xxxx-token-xxxx
    join_params:
        method: "token"|"ec2"|"iam"|"github"|"circleci"|"kubernetes"
        token_name: "token-name"
    ca_pin:
      "sha256:7e12c17c20d9cb504bbcb3f0236be3f446861f1396dcbb44425fe28ec1c108f1"
    advertise_ip: 10.1.0.5
    diag_addr: "127.0.0.1:3000"

    # Only use one of auth_server or proxy_server.
    #
    # When you have either the application service or database service enabled,
    # only tunneling through the proxy is supported, so you should specify proxy_server.
    # All other services support both tunneling through the proxy and directly connecting
    # to the auth server, so you can specify either auth_server or proxy_server.

    # Auth Server address and port to connect to. If you enable the Teleport
    # Auth Server to run in High Availability configuration, the address should
    # point to a Load Balancer.
    # If adding a node located behind NAT, use the Proxy URL (e.g. teleport-proxy.example.com:443)
    # and set 'proxy_server' instead.
    auth_server: 10.1.0.5:3025

    # Proxy Server address and port to connect to. If you enable the Teleport
    # Proxy Server to run in High Availability configuration, the address should
    # point to a Load Balancer.
    proxy_server: %s # this is an inline comment
    log:
        output: /var/lib/teleport/teleport.log
        severity: INFO
        format:
          output: text
          extra_fields: [level, timestamp, component, caller]`

// TestProxyServerDoubleQuotes verifies the upgrader can parse the proxy address
// surrounded by double quotes
func TestProxyServerDoubleQuotes(t *testing.T) {
	endpoint := NewUpgradeEndpoint("/v1/webapi/automaticupgrades/channel/stable/cloud")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up basic test-case that should cause us to fire off an install attempt
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"schedule":               currentSchedule(),
			"state-version-override": "1.2.3", // overrides the upgrader's view of the currently installed teleport version
		},
		agentCfg: fmt.Sprintf(testAgentCfg, fmt.Sprintf("\"localhost:%s\"", port)),
	}

	out, err := tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
}

// TestProxyServerSingleQuotes verifies the upgrader can parse the proxy address
// surrounded by single quotes
func TestProxyServerSingleQuotes(t *testing.T) {
	endpoint := NewUpgradeEndpoint("/v1/webapi/automaticupgrades/channel/stable/cloud")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up basic test-case that should cause us to fire off an install attempt
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"schedule":               currentSchedule(),
			"state-version-override": "1.2.3", // overrides the upgrader's view of the currently installed teleport version
		},
		agentCfg: fmt.Sprintf(testAgentCfg, fmt.Sprintf("'localhost:%s'", port)),
	}

	out, err := tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
}

// TestUpgraderDetectProxyAddr verifies that the updater detects the proxy URL automatically
func TestUpgraderDetectProxyAddr(t *testing.T) {
	endpoint := NewUpgradeEndpoint("/v1/webapi/automaticupgrades/channel/stable/cloud")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up basic test-case that should cause us to fire off an install attempt
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"schedule":               currentSchedule(),
			"state-version-override": "1.2.3", // overrides the upgrader's view of the currently installed teleport version
		},
		agentCfg: fmt.Sprintf(testAgentCfg, fmt.Sprintf("localhost:%s", port)),
	}

	out, err := tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
	require.Equal(t, "reload", nop.restartMode)

	// simulate a successful upgrade by overriding the upgrader's "current version" view to
	// now equal the version served by the endpoint.
	tc.cfg["state-version-override"] = "2.3.4"

	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// expect that no install happened this time
	_, ok = out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)
}

// TestUpgraderHonorOverride checks that the updater honors the override
// even if it can detect the proxy configuration.
func TestUpgraderHonorOverride(t *testing.T) {
	endpoint := NewUpgradeEndpoint("")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up basic test-case that should cause us to fire off an install attempt
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"endpoint":               fmt.Sprintf("localhost:%s/v1/stable/cloud", port),
			"schedule":               currentSchedule(),
			"state-version-override": "1.2.3", // overrides the upgrader's view of the currently installed teleport version
		},
		// This agent config should be ignored because the "endpoint" config is set
		agentCfg: fmt.Sprintf(testAgentCfg, "invalid.localhost:0"),
	}

	out, err := tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
	require.Equal(t, "reload", nop.restartMode)

	// simulate a successful upgrade by overriding the upgrader's "current version" view to
	// now equal the version served by the endpoint.
	tc.cfg["state-version-override"] = "2.3.4"

	out, err = tc.Run()
	require.NoError(t, err)

	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// expect that no install happened this time
	_, ok = out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)
}

func TestUnhealthyAgent(t *testing.T) {
	endpoint := NewUpgradeEndpoint("")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up basic test-case that should cause us to fire off an install attempt
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"endpoint":               fmt.Sprintf("localhost:%s/v1/stable/cloud", port),
			"schedule":               fmt.Sprintf("%d %d", time.Now().Unix()+99, time.Now().Unix()+110),
			"state-version-override": "1.2.3", // overrides the upgrader's view of the currently installed teleport version
		},
	}

	out, err := tc.Run()
	require.NoError(t, err)
	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// not within upgrade window, nothing should happen
	_, ok := out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// set last-restart to the current timestamp to indicate the teleport service is unhealthy
	tc.cfg["state-last-restart"] = time.Now().Format(time.UnixDate)

	// the first run w/ last-restart within a minute should result in us setting the unhealthy marker
	// but not in an actual upgrade attempt.
	out, err = tc.Run()
	require.NoError(t, err)
	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	_, ok = out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// check for expected unhealthy state marker
	us, err := tc.Get("state-unhealthy")
	require.NoError(t, err)
	require.Equal(t, "yes", stripCfgValue(us), "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, us)

	// run again, this time we expect the unhealthy marker state to cause an upgrade
	out, err = tc.Run()
	require.NoError(t, err)
	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)
	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "2.3.4", nop.version)
	// upgrades with an unhealthy schedule state are full restarts
	require.Equal(t, "restart", nop.restartMode)

	// unhealthy marker state should be cleared/removed
	us, err = tc.Get("state-unhealthy")
	require.NoError(t, err)
	require.Equal(t, "", us, "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, us)
}

func TestCache(t *testing.T) {
	endpoint := NewUpgradeEndpoint("")
	endpoint.SetVersion("2.3.4")
	endpoint.SetCritical("no")

	listener, err := net.Listen("tcp4", "localhost:0")
	require.NoError(t, err)

	go endpoint.Serve(listener)
	defer endpoint.Shutdown(context.Background())

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	// set up basic test-case that should cause us to fire off an install attempt
	tc := testCase{
		dir: t.TempDir(),
		cfg: map[string]string{
			"endpoint":               fmt.Sprintf("localhost:%s/v1/stable/cloud", port),
			"schedule":               fmt.Sprintf("%d %d", time.Now().Unix()+99, time.Now().Unix()+110),
			"state-version-override": "1.2.3", // overrides the upgrader's view of the currently installed teleport version
		},
	}

	out, err := tc.Run()
	require.NoError(t, err)
	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	_, ok := out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// state-target-version should be cached
	version, err := tc.Get("state-target-version")
	require.NoError(t, err)
	require.Equal(t, "2.3.4", stripCfgValue(version), "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, version)

	// state-critical should be cached
	critical, err := tc.Get("state-critical")
	require.NoError(t, err)
	require.Equal(t, "no", stripCfgValue(critical), "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, critical)

	// Update version and critical values
	endpoint.SetVersion("3.4.5")
	endpoint.SetCritical("yes")

	out, err = tc.Run()
	require.NoError(t, err)
	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	_, ok = out.GetNopInstall()
	require.False(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	// state-target-version is not expired and should not be modified
	version, err = tc.Get("state-target-version")
	require.NoError(t, err)
	require.Equal(t, "2.3.4", stripCfgValue(version), "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, version)

	// state-critical is not expired and should not be modified
	critical, err = tc.Get("state-critical")
	require.NoError(t, err)
	require.Equal(t, "no", stripCfgValue(critical), "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, critical)

	// Expire cache
	require.NoError(t, tc.Expire("state-target-version"))
	require.NoError(t, tc.Expire("state-critical"))

	out, err = tc.Run()
	require.NoError(t, err)
	require.True(t, out.success, "stdout=%q, stderr=%q", out.stdout, out.stderr)

	nop, ok := out.GetNopInstall()
	require.True(t, ok, "stdout=%q, stderr=%q", out.stdout, out.stderr)
	require.Equal(t, "teleport", nop.target)
	require.Equal(t, "3.4.5", nop.version)

	// state-target-version is reset after an upgrade
	version, err = tc.Get("state-target-version")
	require.NoError(t, err)
	require.Equal(t, "", stripCfgValue(version), "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, version)

	// state-critical is reset after an upgrade
	critical, err = tc.Get("state-critical")
	require.NoError(t, err)
	require.Equal(t, "", stripCfgValue(critical), "stdout=%q, stderr=%q, original=%q", out.stdout, out.stderr, critical)

}
