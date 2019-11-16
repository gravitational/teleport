// +build !linux

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
	"github.com/gravitational/teleport/lib/events"
)

// Service is used on non-Linux systems as a NOP service that allows the
// caller to open and close sessions that do nothing on systems that don't
// support eBPF.
type Service struct {
}

// New returns a new NOP service. Note this function does nothing.
func New(config *Config) (BPF, error) {
	return &NOP{}, nil
}
