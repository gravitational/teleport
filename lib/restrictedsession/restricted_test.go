//go:build bpf && !386
// +build bpf,!386

/*
Copyright 2021 Gravitational, Inc.

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

package restrictedsession

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"syscall"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	api "github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type blockAction int

const (
	allowed = iota
	denied
)

type blockedRange struct {
	ver   int                    // 4 or 6
	deny  string                 // Denied IP range in CIDR format or a lone IP
	allow string                 // Allowed IP range in CIDR format or a lone IP
	probe map[string]blockAction // IP to test the blocked range (needs to be within range)
}

const (
	testPort = 8888
)

var (
	testRanges = []blockedRange{
		{
			ver:   4,
			allow: "39.156.69.70/28",
			deny:  "39.156.69.71",
			probe: map[string]blockAction{
				"39.156.69.64": allowed,
				"39.156.69.79": allowed,
				"39.156.69.71": denied,
				"39.156.69.63": denied,
				"39.156.69.80": denied,
				"72.156.69.80": denied,
			},
		},
		{
			ver:   4,
			allow: "77.88.55.88",
			probe: map[string]blockAction{
				"77.88.55.88": allowed,
				"77.88.55.87": denied,
				"77.88.55.86": denied,
				"67.88.55.86": denied,
			},
		},
		{
			ver:   6,
			allow: "39.156.68.48/28",
			deny:  "39.156.68.48/31",
			probe: map[string]blockAction{
				"::ffff:39.156.68.48": denied,
				"::ffff:39.156.68.49": denied,
				"::ffff:39.156.68.50": allowed,
				"::ffff:39.156.68.63": allowed,
				"::ffff:39.156.68.47": denied,
				"::ffff:39.156.68.64": denied,
				"::ffff:72.156.68.80": denied,
			},
		},
		{
			ver:   6,
			allow: "fc80::/64",
			deny:  "fc80::10/124",
			probe: map[string]blockAction{
				"fc80::":                    allowed,
				"fc80::ffff:ffff:ffff:ffff": allowed,
				"fc80::10":                  denied,
				"fc80::1f":                  denied,
				"fc7f:ffff:ffff:ffff:ffff:ffff:ffff:ffff": denied,
				"fc60:0:0:1::": denied,
			},
		},
		{
			ver:   6,
			allow: "2607:f8b0:4005:80a::200e",
			probe: map[string]blockAction{
				"2607:f8b0:4005:80a::200e": allowed,
				"2607:f8b0:4005:80a::200d": denied,
				"2607:f8b0:4005:80a::200f": denied,
				"2607:f8b0:4005:80a::300f": denied,
			},
		},
	}
)

type bpfContext struct {
	cgroupDir        string
	cgroupID         uint64
	ctx              *bpf.SessionContext
	enhancedRecorder bpf.BPF
	restrictedMgr    Manager
	srcAddrs         map[int]string

	// Audit events emitted by us
	emitter             eventstest.MockEmitter
	expectedAuditEvents []apievents.AuditEvent
}

func setupBPFContext(t *testing.T) *bpfContext {
	utils.InitLoggerForTests()

	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	if !isRoot() {
		t.Skip("Tests for package restrictedsession can only be run as root.")
	}

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	// DO NOT MOVE THIS LINE.
	// t.TempDir() creates t.Cleanup() action. If this hook is called before
	// we mount cgroup in Close() this test will fail.
	cgroupDir := t.TempDir()

	bpfCtx := bpfContext{
		cgroupDir: cgroupDir,
	}
	t.Cleanup(func() { bpfCtx.Close(t) })

	bpfCtx.srcAddrs = map[int]string{
		4: "0.0.0.0",
		6: "::",
	}

	config := &servicecfg.RestrictedSessionConfig{
		Enabled: true,
	}

	var err error
	// Create BPF service since we piggyback on it
	bpfCtx.enhancedRecorder, err = bpf.New(&servicecfg.BPFConfig{
		Enabled:    true,
		CgroupPath: bpfCtx.cgroupDir,
	}, config)
	require.NoError(t, err)

	// Create the SessionContext used by both enhanced recording and us (restricted session)
	bpfCtx.ctx = &bpf.SessionContext{
		Namespace:      apidefaults.Namespace,
		SessionID:      uuid.New().String(),
		ServerID:       uuid.New().String(),
		ServerHostname: "ip-172-31-11-148",
		Login:          "foo",
		User:           "foo@example.com",
		PID:            os.Getpid(),
		Emitter:        &bpfCtx.emitter,
		Events:         map[string]bool{},
	}

	// Create enhanced recording session to piggyback on.
	bpfCtx.cgroupID, err = bpfCtx.enhancedRecorder.OpenSession(bpfCtx.ctx)
	require.NoError(t, err)
	require.Equal(t, bpfCtx.cgroupID > 0, true)

	deny := []api.AddressCondition{}
	allow := []api.AddressCondition{}
	for _, r := range testRanges {
		if len(r.deny) > 0 {
			deny = append(deny, api.AddressCondition{CIDR: r.deny})
		}

		if len(r.allow) > 0 {
			allow = append(allow, api.AddressCondition{CIDR: r.allow})
		}
	}

	restrictions := api.NewNetworkRestrictions()
	restrictions.SetAllow(allow)
	restrictions.SetDeny(deny)

	client := &mockClient{
		restrictions: restrictions,
		Fanout:       *services.NewFanout(),
	}

	bpfCtx.restrictedMgr, err = New(config, client)
	require.NoError(t, err)

	client.Fanout.SetInit([]api.WatchKind{{Kind: api.KindNetworkRestrictions}})

	time.Sleep(100 * time.Millisecond)

	return &bpfCtx
}

func (tt *bpfContext) Close(t *testing.T) {
	if tt.cgroupID > 0 {
		tt.restrictedMgr.CloseSession(tt.ctx, tt.cgroupID)
	}

	if tt.restrictedMgr != nil {
		tt.restrictedMgr.Close()
	}

	if tt.enhancedRecorder != nil && tt.ctx != nil {
		err := tt.enhancedRecorder.CloseSession(tt.ctx)
		require.NoError(t, err)
		const restarting = false
		err = tt.enhancedRecorder.Close(restarting)
		require.NoError(t, err)
	}

	if tt.cgroupDir != "" {
		err := os.RemoveAll(tt.cgroupDir)
		require.NoError(t, err)
	}
}

func (tt *bpfContext) openSession(t *testing.T) {
	// Create the restricted session
	tt.restrictedMgr.OpenSession(tt.ctx, tt.cgroupID)
}

func (tt *bpfContext) closeSession(t *testing.T) {
	// Close the restricted session
	tt.restrictedMgr.CloseSession(tt.ctx, tt.cgroupID)
}

func (tt *bpfContext) dialExpectAllow(t *testing.T, ver int, ip string) {
	if err := dialTCP(ver, mustParseIP(ip)); err != nil {
		// Other than EPERM or EINVAL is OK
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EINVAL) {
			t.Fatalf("Dial %v was not allowed: %v", ip, err)
		}
	}
}

func (tt *bpfContext) sendExpectAllow(t *testing.T, ver int, ip string) {
	err := sendUDP(ver, mustParseIP(ip))

	// Other than EPERM or EINVAL is OK
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EINVAL) {
		t.Fatalf("Send %v: failed with %v", ip, err)
	}
}

func (tt *bpfContext) dialExpectDeny(t *testing.T, ver int, ip string) {
	// Only EPERM is expected
	err := dialTCP(ver, mustParseIP(ip))
	if err == nil {
		t.Fatalf("Dial %v: did not expect to succeed", ip)
	}

	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("Dial %v: EPERM expected, got: %v", ip, err)
	}

	ev := tt.expectedAuditEvent(ver, ip, apievents.SessionNetwork_CONNECT)
	tt.expectedAuditEvents = append(tt.expectedAuditEvents, ev)
}

func (tt *bpfContext) sendExpectDeny(t *testing.T, ver int, ip string) {
	err := sendUDP(ver, mustParseIP(ip))
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("Send %v: was not denied: %v", ip, err)
	}

	ev := tt.expectedAuditEvent(ver, ip, apievents.SessionNetwork_SEND)
	tt.expectedAuditEvents = append(tt.expectedAuditEvents, ev)
}

func (tt *bpfContext) expectedAuditEvent(ver int, ip string, op apievents.SessionNetwork_NetworkOperation) apievents.AuditEvent {
	return &apievents.SessionNetwork{
		Metadata: apievents.Metadata{
			Type: events.SessionNetworkEvent,
			Code: events.SessionNetworkCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        tt.ctx.ServerID,
			ServerHostname:  tt.ctx.ServerHostname,
			ServerNamespace: tt.ctx.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: tt.ctx.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User:  tt.ctx.User,
			Login: tt.ctx.Login,
		},
		BPFMetadata: apievents.BPFMetadata{
			CgroupID: tt.cgroupID,
			Program:  "restrictedsessi",
			PID:      uint64(tt.ctx.PID),
		},
		DstPort:    testPort,
		DstAddr:    ip,
		SrcAddr:    tt.srcAddrs[ver],
		TCPVersion: int32(ver),
		Operation:  op,
		Action:     apievents.EventAction_DENIED,
	}
}

func TestRootNetwork(t *testing.T) {
	if !bpfTestEnabled() {
		t.Skip("BPF testing is disabled")
	}

	tt := setupBPFContext(t)

	type testCase struct {
		ver      int
		ip       string
		expected blockAction
	}

	tests := []testCase{}
	for _, r := range testRanges {
		for ip, expected := range r.probe {
			tests = append(tests, testCase{
				ver:      r.ver,
				ip:       ip,
				expected: expected,
			})
		}
	}

	// Restricted session is not yet open, all these should be allowed
	for _, tc := range tests {
		tt.dialExpectAllow(t, tc.ver, tc.ip)
		tt.sendExpectAllow(t, tc.ver, tc.ip)
	}

	// Nothing should be reported to the audit log
	time.Sleep(100 * time.Millisecond)
	require.Empty(t, tt.emitter.Events())

	// Open the restricted session
	tt.openSession(t)

	// Now the policy should be enforced
	for _, tc := range tests {
		if tc.expected == denied {
			tt.dialExpectDeny(t, tc.ver, tc.ip)
			tt.sendExpectDeny(t, tc.ver, tc.ip)
		} else {
			tt.dialExpectAllow(t, tc.ver, tc.ip)
			tt.sendExpectAllow(t, tc.ver, tc.ip)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Close the restricted session
	tt.closeSession(t)

	// Check that the emitted audit events are correct
	actualAuditEvents := tt.emitter.Events()
	require.Empty(t, gocmp.Diff(tt.expectedAuditEvents, actualAuditEvents),
		"Audit events mismatch (-want +got)")

	// Clear out the expected and actual events
	tt.expectedAuditEvents = nil
	tt.emitter.Reset()

	// Restricted session is now closed, all these should be allowed
	for _, tc := range tests {
		tt.dialExpectAllow(t, tc.ver, tc.ip)
		tt.sendExpectAllow(t, tc.ver, tc.ip)
	}

	// Nothing should be reported to the audit log
	time.Sleep(100 * time.Millisecond)
	require.Empty(t, tt.emitter.Events())
}

type mockClient struct {
	restrictions api.NetworkRestrictions
	services.Fanout
}

func (mc *mockClient) GetNetworkRestrictions(context.Context) (api.NetworkRestrictions, error) {
	return mc.restrictions, nil
}

func (_ *mockClient) SetNetworkRestrictions(context.Context, api.NetworkRestrictions) error {
	return nil
}

func (_ *mockClient) DeleteNetworkRestrictions(context.Context) error {
	return nil
}

var ip4Regex = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)

// mustParseIP parses the IP and also converts IPv4 addresses
// to 4 byte representation. IPv4 mapped (into IPv6) addresses
// are kept in 16 byte encoding
func mustParseIP(addr string) net.IP {
	is4 := ip4Regex.MatchString(addr)

	ip := net.ParseIP(addr)
	if is4 {
		return ip.To4()
	}
	return ip.To16()
}

func testSocket(ver, typ int, ip net.IP) (int, syscall.Sockaddr, error) {
	var domain int
	var src syscall.Sockaddr
	var dst syscall.Sockaddr
	if ver == 4 {
		domain = syscall.AF_INET
		src = &syscall.SockaddrInet4{}
		dst4 := &syscall.SockaddrInet4{
			Port: testPort,
		}
		copy(dst4.Addr[:], ip)
		dst = dst4
	} else {
		domain = syscall.AF_INET6
		src = &syscall.SockaddrInet6{}
		dst6 := &syscall.SockaddrInet6{
			Port: testPort,
		}
		copy(dst6.Addr[:], ip)
		dst = dst6
	}

	fd, err := syscall.Socket(domain, typ, 0)
	if err != nil {
		return 0, nil, fmt.Errorf("socket() failed: %v", err)
	}

	if err = syscall.Bind(fd, src); err != nil {
		return 0, nil, fmt.Errorf("bind() failed: %v", err)
	}

	return fd, dst, nil
}

func dialTCP(ver int, ip net.IP) error {
	fd, dst, err := testSocket(ver, syscall.SOCK_STREAM, ip)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	tv := syscall.Timeval{
		Usec: 1000,
	}
	err = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv)
	if err != nil {
		return fmt.Errorf("setsockopt(SO_SNDTIMEO) failed: %v", err)
	}

	return syscall.Connect(fd, dst)
}

func sendUDP(ver int, ip net.IP) error {
	fd, dst, err := testSocket(ver, syscall.SOCK_DGRAM, ip)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	return syscall.Sendto(fd, []byte("abc"), 0, dst)
}

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func isRoot() bool {
	return os.Geteuid() == 0
}

// bpfTestEnabled returns true if BPF/LSM tests should run. Tests can be enabled by
// setting TELEPORT_BPF_LSM_TEST environment variable to any value.
func bpfTestEnabled() bool {
	return os.Getenv("TELEPORT_BPF_LSM_TEST") != ""
}
