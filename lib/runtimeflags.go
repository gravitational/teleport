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

This file contains a single global variable controlling which edition
of Teleport is running

This flag contains various global booleans that are set during
Teleport initialization.

These are NOT for configuring Teleport: use regular Config facilities for that,
preferrably tailored to specific services, i.e proxy config, auth config, etc

These are for being set once, at the beginning of the process, and for
being visible to any code under 'lib'

*/

package lib

import (
	"sync"

	"github.com/gravitational/teleport"
)

var (
	// currentDistroType contains the type of teleport binary: enterprise or
	// open source this flag does not enable any enterprise features, but it
	// makes the default experience of the OSS users nicer.
	currentDistroType teleport.DistroType = teleport.DistroTypeOSS

	// insecureDevMode is set to 'true' when teleport is started with a hidden
	// --insecure flag. This mode is only useful for learning Teleport and following
	// quick starts: it disables HTTPS certificate validation
	insecureDevMode bool

	// flagLock protects access to all globals declared in this file
	flagLock sync.Mutex
)

func SetDistroType(t teleport.DistroType) {
	flagLock.Lock()
	defer flagLock.Unlock()
	currentDistroType = t
}

// IsEnterprise returns 'true' if Teleport is packaged with enterprise runime
func IsEnterprise() bool {
	flagLock.Lock()
	defer flagLock.Unlock()
	return currentDistroType == teleport.DistroTypeEnterprise
}

// SetInsecureDevMode turns the 'insecure' mode on. In this mode Teleport accpets
// self-signed HTTPS certificates (for development only!)
func SetInsecureDevMode(m bool) {
	flagLock.Lock()
	defer flagLock.Unlock()
	insecureDevMode = m
}

// IsInsecureDevMode returns 'true' if Teleport daemon was started with the
// --insecure flag
func IsInsecureDevMode() bool {
	flagLock.Lock()
	defer flagLock.Unlock()
	return insecureDevMode
}
