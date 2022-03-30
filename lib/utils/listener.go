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

package utils

import (
	"net"
	"os"

	"github.com/gravitational/trace"
)

// GetListenerFile returns file associated with listener
func GetListenerFile(listener net.Listener) (*os.File, error) {
	switch t := listener.(type) {
	case *net.TCPListener:
		return t.File()
	case *net.UnixListener:
		return t.File()
	}
	return nil, trace.BadParameter("unsupported listener: %T", listener)
}

// CloseListener closes provided listener and ignores errors that are normal
// connection close.
func CloseListener(listener net.Listener) error {
	if listener == nil {
		return nil
	}

	err := listener.Close()
	if err != nil && !IsOKNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}
