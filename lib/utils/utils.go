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
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"golang.org/x/crypto/ssh"
)

// SplitHostPort splits host and port and checks that host is not empty
func SplitHostPort(hostname string) (string, string, error) {
	host, port, err := net.SplitHostPort(hostname)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	if host == "" {
		return "", "", trace.BadParameter("empty hostname")
	}
	return host, port, nil
}

type HostKeyCallback func(hostID string, remote net.Addr, key ssh.PublicKey) error

func ReadPath(path string) ([]byte, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	abs, err := filepath.EvalSymlinks(s)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	bytes, err := ioutil.ReadFile(abs)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return bytes, nil
}

type multiCloser struct {
	closers []io.Closer
}

func (mc *multiCloser) Close() error {
	for _, closer := range mc.closers {
		if err := closer.Close(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// MultiCloser implements io.Close, it sequentially calls Close() on each object
func MultiCloser(closers ...io.Closer) *multiCloser {
	return &multiCloser{
		closers: closers,
	}
}

// IsHandshakeFailedError specifies whether this error indicates
// failed handshake
func IsHandshakeFailedError(err error) bool {
	return strings.Contains(trace.Unwrap(err).Error(), "ssh: handshake failed")
}

// IsShellFailedError specifies whether this error indicates
// failed attempt to start shell
func IsShellFailedError(err error) bool {
	return strings.Contains(err.Error(), "ssh: cound not start shell")
}

// PortList is a list of TCP port
type PortList []string

// Pop returns a value from the list, it panics if the value is not there
func (p *PortList) Pop() string {
	if len(*p) == 0 {
		panic("list is empty")
	}
	val := (*p)[len(*p)-1]
	*p = (*p)[:len(*p)-1]
	return val
}

// GetFreeTCPPorts returns a lit of available ports on localhost
// used for testing
func GetFreeTCPPorts(n int) (PortList, error) {
	list := make(PortList, 0, n)
	for i := 0; i < n; i++ {
		addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		listener, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer listener.Close()
		tcpAddr, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			return nil, trace.Errorf("Can't get tcp address")
		}
		list = append(list, strconv.Itoa(tcpAddr.Port))
	}
	return list, nil
}

// ReadHostUUID reads host UUID from the file in the data dir
func ReadHostUUID(dataDir string) (string, error) {
	out, err := ReadPath(filepath.Join(dataDir, HostUUIDFile))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(out), nil
}

// WriteHostUUID writes host UUID into a file
func WriteHostUUID(dataDir string, id string) error {
	err := ioutil.WriteFile(filepath.Join(dataDir, HostUUIDFile), []byte(id), os.ModeExclusive|0400)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// ReadOrMakeHostUUID looks for a hostid file in the data dir. If present,
// returns the UUID from it, otherwise generates one
func ReadOrMakeHostUUID(dataDir string) (string, error) {
	id, err := ReadHostUUID(dataDir)
	if err == nil {
		return id, nil
	}
	if !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	id = uuid.New()
	if err = WriteHostUUID(dataDir, id); err != nil {
		return "", trace.Wrap(err)
	}
	return id, nil
}

// PrintVersion prints human readable version.
//   - distro: name of the distribution. Empty string for OSS or "enterprise"
func PrintVersion(distro teleport.DistroType) {
	if distro == teleport.DistroTypeEnterprise {
		distro = " " + distro
	} else {
		distro = ""
	}
	ver := fmt.Sprintf("Teleport%s v%s", distro, teleport.Version)
	if teleport.Gitref != "" {
		ver = fmt.Sprintf("%s git:%s", ver, teleport.Gitref)
	}
	fmt.Println(ver)
}

// HumanTimeFormat formats time as recognized by humans
func HumanTimeFormat(d time.Time) string {
	return d.Format(HumanTimeFormatString)
}

// Deduplicate deduplicates list of strings
func Deduplicate(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, val := range in {
		if _, ok := seen[val]; !ok {
			out = append(out, val)
			seen[val] = true
		}
	}
	return out
}

// SliceContainsStr returns 'true' if the slice contains the given value
func SliceContainsStr(slice []string, value string) bool {
	for i := range slice {
		if slice[i] == value {
			return true
		}
	}
	return false
}

// CheckCompatibilityFlag check that the compatibility flag is valid.
func CheckCompatibilityFlag(s string) (string, error) {
	switch s {
	case teleport.CompatibilityNone, teleport.CompatibilityOldSSH:
		return s, nil
	default:
		return teleport.CompatibilityNone, trace.BadParameter("invalid compatibility parameter: %q", s)
	}
}

const (
	// HumanTimeFormatString is a human readable date formatting
	HumanTimeFormatString = "Mon Jan _2 15:04 UTC"
	// CertTeleportUser specifies teleport user
	CertTeleportUser = "x-teleport-user"
	// CertTeleportUserCA specifies teleport certificate authority
	CertTeleportUserCA = "x-teleport-user-ca"
	// CertExtensionRole specifies teleport role
	CertExtensionRole = "x-teleport-role"
	// CertExtensionAuthority specifies teleport authority's name
	// that signed this domain
	CertExtensionAuthority = "x-teleport-authority"
	// HostUUIDFile is the file name where the host UUID file is stored
	HostUUIDFile = "host_uuid"
	// CertTeleportClusterName  is a name of the teleport cluster
	CertTeleportClusterName = "x-teleport-cluster-name"
)
