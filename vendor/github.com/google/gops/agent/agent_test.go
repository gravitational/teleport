// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package agent

import (
	"os"
	"testing"
)

func TestListen(t *testing.T) {
	err := Listen(nil)
	if err != nil {
		t.Fatal(err)
	}
	Close()
}

func TestAgentClose(t *testing.T) {
	err := Listen(nil)
	if err != nil {
		t.Fatal(err)
	}
	Close()
	_, err = os.Stat(portfile)
	if !os.IsNotExist(err) {
		t.Fatalf("portfile = %q doesn't exist; err = %v", portfile, err)
	}
	if portfile != "" {
		t.Fatalf("got = %q; want empty portfile", portfile)
	}
}

func TestAgentListenMultipleClose(t *testing.T) {
	err := Listen(nil)
	if err != nil {
		t.Fatal(err)
	}
	Close()
	Close()
	Close()
	Close()
}
