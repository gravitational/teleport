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
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"syscall"
	"testing"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	api "github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
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
		blockedRange{
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
		blockedRange{
			ver:   4,
			allow: "77.88.55.88",
			probe: map[string]blockAction{
				"77.88.55.88": allowed,
				"77.88.55.87": denied,
				"77.88.55.86": denied,
				"67.88.55.86": denied,
			},
		},
		blockedRange{
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
		blockedRange{
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
		blockedRange{
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

type Suite struct {
	cgroupDir        string
	cgroupID         uint64
	ctx              *bpf.SessionContext
	enhancedRecorder bpf.BPF
	restrictedMgr    Manager
	srcAddrs         map[int]string

	// Audit events emitted by us
	emitter             events.MockEmitter
	expectedAuditEvents []apievents.AuditEvent
}

var _ = check.Suite(&Suite{})

func TestRootRestrictedSession(t *testing.T) { check.TestingT(t) }

func mustParseIPSpec(cidr string) *net.IPNet {
	ipnet, err := ParseIPSpec(cidr)
	if err != nil {
		panic(err)
	}
	return ipnet
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

func (s *Suite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	// This test must be run as root and the host has to be capable of running
	// BPF programs.
	if !isRoot() {
		c.Skip("Tests for package restrictedsession can only be run as root.")
	}
	err := bpf.IsHostCompatible()
	if err != nil {
		c.Skip(fmt.Sprintf("Tests for package restrictedsession can not be run: %v.", err))
	}

	s.srcAddrs = map[int]string{
		4: "0.0.0.0",
		6: "::",
	}

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	s.cgroupDir, err = ioutil.TempDir("", "cgroup-test")
	c.Assert(err, check.IsNil)

	// Create BPF service since we piggy-back on it
	s.enhancedRecorder, err = bpf.New(&bpf.Config{
		Enabled:    true,
		CgroupPath: s.cgroupDir,
	})
	c.Assert(err, check.IsNil)

	// Create the SessionContext used by both enhanced recording and us (restricted session)
	s.ctx = &bpf.SessionContext{
		Namespace: apidefaults.Namespace,
		SessionID: uuid.New(),
		ServerID:  uuid.New(),
		Login:     "foo",
		User:      "foo@example.com",
		PID:       os.Getpid(),
		Emitter:   &s.emitter,
		Events:    map[string]bool{},
	}

	// Create enhanced recording session to piggy-back on.
	s.cgroupID, err = s.enhancedRecorder.OpenSession(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(s.cgroupID > 0, check.Equals, true)

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

	config := &Config{
		Enabled: true,
	}

	client := &mockClient{
		restrictions: restrictions,
		Fanout:       *services.NewFanout(),
	}

	s.restrictedMgr, err = New(config, client)
	c.Assert(err, check.IsNil)

	client.Fanout.SetInit()

	time.Sleep(100 * time.Millisecond)
}

func (s *Suite) TearDownSuite(c *check.C) {
	if s.restrictedMgr != nil {
		s.restrictedMgr.Close()
	}

	if s.enhancedRecorder != nil && s.ctx != nil {
		err := s.enhancedRecorder.CloseSession(s.ctx)
		c.Assert(err, check.IsNil)
	}

	if s.cgroupDir != "" {
		os.RemoveAll(s.cgroupDir)
	}
}

func (s *Suite) TearDownTest(c *check.C) {
	if s.cgroupID > 0 {
		s.restrictedMgr.CloseSession(s.ctx, s.cgroupID)
	}
}

func (s *Suite) openSession(c *check.C) {
	// Create the restricted session
	s.restrictedMgr.OpenSession(s.ctx, s.cgroupID)
}

func (s *Suite) closeSession(c *check.C) {
	// Close the restricted session
	s.restrictedMgr.CloseSession(s.ctx, s.cgroupID)
}

var ip4Regex = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)

// mustParseIP parses the IP and also converts IPv4 addresses
// to 4 byte represenetation. IPv4 mapped (into IPv6) addresses
// are kept in 16 byte encoding
func mustParseIP(addr string) net.IP {
	is4 := ip4Regex.MatchString(addr)

	ip := net.ParseIP(addr)
	if is4 {
		return ip.To4()
	} else {
		return ip.To16()
	}
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

func (s *Suite) dialExpectAllow(c *check.C, ver int, ip string) {
	if err := dialTCP(ver, mustParseIP(ip)); err != nil {
		// Other than EPERM or EINVAL is OK
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EINVAL) {
			c.Fatalf("Dial %v was not allowed: %v", ip, err)
		}
	}
}

func (s *Suite) dialExpectDeny(c *check.C, ver int, ip string) {
	// Only EPERM is expected
	err := dialTCP(ver, mustParseIP(ip))
	if err == nil {
		c.Fatalf("Dial %v: did not expect to succeed", ip)
	}

	if !errors.Is(err, syscall.EPERM) {
		c.Fatalf("Dial %v: EPERM expected, got: %v", ip, err)
	}

	ev := s.expectedAuditEvent(ver, ip, apievents.SessionNetwork_CONNECT)
	s.expectedAuditEvents = append(s.expectedAuditEvents, ev)
}

func (s *Suite) sendExpectAllow(c *check.C, ver int, ip string) {
	err := sendUDP(ver, mustParseIP(ip))

	// Other than EPERM or EINVAL is OK
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EINVAL) {
		c.Fatalf("Send %v: failed with %v", ip, err)
	}
}

func (s *Suite) sendExpectDeny(c *check.C, ver int, ip string) {
	err := sendUDP(ver, mustParseIP(ip))
	if !errors.Is(err, syscall.EPERM) {
		c.Fatalf("Send %v: was not denied: %v", ip, err)
	}

	ev := s.expectedAuditEvent(ver, ip, apievents.SessionNetwork_SEND)
	s.expectedAuditEvents = append(s.expectedAuditEvents, ev)
}

func (s *Suite) expectedAuditEvent(ver int, ip string, op apievents.SessionNetwork_NetworkOperation) apievents.AuditEvent {
	return &apievents.SessionNetwork{
		Metadata: apievents.Metadata{
			Type: events.SessionNetworkEvent,
			Code: events.SessionNetworkCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        s.ctx.ServerID,
			ServerNamespace: s.ctx.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: s.ctx.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User:  s.ctx.User,
			Login: s.ctx.Login,
		},
		BPFMetadata: apievents.BPFMetadata{
			CgroupID: s.cgroupID,
			Program:  "restrictedsessi",
			PID:      uint64(s.ctx.PID),
		},
		DstPort:    testPort,
		DstAddr:    ip,
		SrcAddr:    s.srcAddrs[ver],
		TCPVersion: int32(ver),
		Operation:  op,
		Action:     apievents.EventAction_DENIED,
	}
}

func (s *Suite) TestNetwork(c *check.C) {
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
	for _, t := range tests {
		s.dialExpectAllow(c, t.ver, t.ip)
		s.sendExpectAllow(c, t.ver, t.ip)
	}

	// Nothing should be reported to the audit log
	time.Sleep(100 * time.Millisecond)
	c.Assert(s.emitter.Events(), check.HasLen, 0)

	// Open the restricted session
	s.openSession(c)

	// Now the policy should be enforced
	for _, t := range tests {
		if t.expected == denied {
			s.dialExpectDeny(c, t.ver, t.ip)
			s.sendExpectDeny(c, t.ver, t.ip)
		} else {
			s.dialExpectAllow(c, t.ver, t.ip)
			s.sendExpectAllow(c, t.ver, t.ip)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Close the restricted session
	s.closeSession(c)

	// Check that the emitted audit events are correct
	actualAuditEvents := s.emitter.Events()
	c.Assert(actualAuditEvents, check.DeepEquals, s.expectedAuditEvents)

	// Clear out the expected and actual evetns
	s.expectedAuditEvents = nil
	s.emitter.Reset()

	// Restricted session is now closed, all these should be allowed
	for _, t := range tests {
		s.dialExpectAllow(c, t.ver, t.ip)
		s.sendExpectAllow(c, t.ver, t.ip)
	}

	// Nothing should be reported to the audit log
	time.Sleep(100 * time.Millisecond)
	c.Assert(s.emitter.Events(), check.HasLen, 0)
}

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func isRoot() bool {
	return os.Geteuid() == 0
}
