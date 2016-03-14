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
// Package tool provides a currying wrapper around os/exec.Command that fixes a set of arguments.
package tool

import (
	"bytes"
	"fmt"
	"os/exec"
)

// T represents an instance of a running tool specified with cmd and
// an optional list of fixed initial arguments args.
type T struct {
	Cmd  string
	Args []string
}

// Error is a tool execution error.
type Error struct {
	Tool   string
	Output []byte
	Err    error
}

func (r *Error) Error() string {
	return fmt.Sprintf("error executing `%s`: %v (%s)", r.Tool, r.Err, r.Output)
}

// exec executes a given command specified with args prepending a set of fixed arguments.
// Otherwise behaves exactly as rawExec
func (r *T) Exec(args ...string) (string, error) {
	args = append(r.Args[:], args...)
	return r.RawExec(args...)
}

// RawExec executes a given command specified with args and returns the output
// with whitespace trimmed.
func (r *T) RawExec(args ...string) (string, error) {
	out, err := exec.Command(r.Cmd, args...).CombinedOutput()
	if err == nil {
		out = bytes.TrimSpace(out)
	}
	if err != nil {
		err = &Error{
			Tool:   r.Cmd,
			Output: out,
			Err:    err,
		}
	}
	return string(out), err
}
