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

	"github.com/gravitational/teleport"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/pborman/uuid"
	"golang.org/x/crypto/ssh"
)

type HostKeyCallback func(hostID string, remote net.Addr, key ssh.PublicKey) error

func ReadPath(path string) ([]byte, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return nil, trace.Wrap(teleport.ConvertSystemError(err))
	}
	abs, err := filepath.EvalSymlinks(s)
	if err != nil {
		return nil, trace.Wrap(teleport.ConvertSystemError(err))
	}
	bytes, err := ioutil.ReadFile(abs)
	if err != nil {
		return nil, trace.Wrap(teleport.ConvertSystemError(err))
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
	return strings.Contains(err.Error(), "ssh: handshake failed")
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

// ReadOrMakeHostUUID looks for a hostid file in the data dir. If present,
// returns the UUID from it, otherwise generates one
func ReadOrMakeHostUUID(dataDir string) (string, error) {
	const ModeReadonly = 0400

	fp := filepath.Join(dataDir, HostUUIDFile)
	bytes, err := ioutil.ReadFile(fp)
	if err != nil {
		// uuid file not found? re-create it
		if os.IsNotExist(err) {
			bytes = []byte(uuid.New())
			err = ioutil.WriteFile(fp, bytes, os.ModeExclusive|ModeReadonly)
		}
	}
	if err != nil {
		if os.IsPermission(err) {
			return "", trace.Errorf("permission error trying to access %v", fp)
		}
		log.Errorf("failed generating host UUID. %v", err)
		return "", trace.Wrap(err)
	}
	return string(bytes), nil
}

// PrintVersion prints human readable version
func PrintVersion() {
	ver := version.Get()
	if ver.GitCommit != "" {
		fmt.Printf("%v git:%v\n", ver.Version, ver.GitCommit)
	} else {
		fmt.Printf("%v\n", ver.Version)
	}
}

const (
	// CertTeleportUser specifies teleport user
	CertTeleportUser = "x-teleport-user"
	// CertExtensionRole specifies teleport role
	CertExtensionRole = "x-teleport-role"
	// CertExtensionAuthority specifies teleport authority's name
	// that signed this domain
	CertExtensionAuthority = "x-teleport-authority"
	// HostUUIDFile is the file name where the host UUID file is stored
	HostUUIDFile = "host_uuid"
)
