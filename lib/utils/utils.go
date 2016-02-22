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
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

type HostKeyCallback func(hostname string, remote net.Addr, key ssh.PublicKey) error

func ReadPath(path string) ([]byte, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path %v, err %v", s, err)
	}
	abs, err := filepath.EvalSymlinks(s)
	if err != nil {
		return nil, fmt.Errorf("failed to eval symlinks in path %v, err %v", path, err)
	}
	bytes, err := ioutil.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func WriteArchive(root_directory string, w io.Writer) error {
	ar := tar.NewWriter(w)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsDir() {
			return nil
		}
		// Because of scoping we can reference the external root_directory variable
		new_path := path[len(root_directory):]
		if len(new_path) == 0 {
			return nil
		}
		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		if h, err := tar.FileInfoHeader(info, new_path); err != nil {
			return err
		} else {
			h.Name = new_path
			if err = ar.WriteHeader(h); err != nil {
				return err
			}
		}
		if length, err := io.Copy(ar, fr); err != nil {
			return err
		} else {
			fmt.Println(length)
		}
		return nil
	}

	return filepath.Walk(root_directory, walkFn)
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

// MultiCloser implements io.Close,
// it sequentially calls Close() on each object
func MultiCloser(closers ...io.Closer) *multiCloser {
	return &multiCloser{
		closers: closers,
	}
}

// IsHandshakeFailedError specifies whether this error indicates
// failed handshake
func IsHandshakeFailedError(err error) bool {
	return strings.Contains(err.Error(), "handshake failed")
}

func GetFreeTCPPort() (port string, e error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", trace.Wrap(err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer listener.Close()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return "", trace.Errorf("Can't get tcp address")
	}
	return strconv.Itoa(tcpAddr.Port), nil
}

const (
	// CertExtensionUser specifies teleport specific user entry
	CertExtensionUser = "x-teleport-user"
	// CertExtensionRole specifies teleport role
	CertExtensionRole = "x-teleport-role"
	// CertExtensionAuthority specifies teleport authority's name
	// that signed this domain
	CertExtensionAuthority = "x-teleport-authority"
)
