/*
Copyright 2017 Gravitational, Inc.

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

package srv

import (
	"golang.org/x/crypto/ssh"
)

// SubsystemResult is a result of execution of the subsystem.
type SubsystemResult struct {
	// Name holds the name of the subsystem that was executed.
	Name string

	// Err holds the result of execution of the subsystem.
	Err error
}

// Subsystem represents SSH subsytem - special command executed
// in the context of the session.
type Subsystem interface {
	// Start starts subsystem
	Start(*ssh.ServerConn, ssh.Channel, *ssh.Request, *ServerContext) error

	// Wait is returned by subystem when it's completed
	Wait() error
}
