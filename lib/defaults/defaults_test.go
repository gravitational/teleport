/*
Copyright 2016 Gravitational, Inc.

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
package defaults

import (
	"github.com/gravitational/teleport/lib/utils"
	"testing"
)

func TestMakeAddr(t *testing.T) {
	addr := makeAddr("example.com", 3022)
	if addr == nil {
		t.Fatal("makeAddr failed")
	}
	if addr.FullAddress() != "tcp://example.com:3022" {
		t.Fatalf("makeAddr did not make a correct address. Got: %v", addr.FullAddress())
	}
}

func TestDefaultAddresses(t *testing.T) {
	table := map[string]*utils.NetAddr{
		"tcp://0.0.0.0:3025":   AuthListenAddr(),
		"tcp://127.0.0.1:3025": AuthConnectAddr(),
		"tcp://0.0.0.0:3023":   ProxyListenAddr(),
		"tcp://0.0.0.0:3080":   ProxyWebListenAddr(),
		"tcp://0.0.0.0:3022":   SSHServerListenAddr(),
		"tcp://0.0.0.0:3024":   ReverseTunnellListenAddr(),
	}
	for expected, actual := range table {
		if actual == nil {
			t.Fatalf("Expected '%v' got nil", expected)
		} else if actual.FullAddress() != expected {
			t.Errorf("Expected '%v' got '%v'", expected, actual.FullAddress())
		}
	}
}
