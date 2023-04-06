/*
Copyright 2023 Gravitational, Inc.

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
	"os/user"
	"runtime"
	"time"

	"github.com/gravitational/trace"
)

const lookupTimeout = 10 * time.Second

type userResult struct {
	user *user.User
	err  error
}

// CurrentUser is just like [user.Current], except that on Windows an
// error is returned if the user lookup exceeds 10 seconds. This is
// because if [user.Current] is called on a domain joined host, the
// user lookup will contact potentially multiple domain controllers
// which can hang.
func CurrentUser() (*user.User, error) {
	if runtime.GOOS != "windows" {
		return user.Current()
	}

	result := make(chan userResult)
	go func() {
		u, err := user.Current()
		result <- userResult{
			user: u,
			err:  err,
		}
	}()

	select {
	case u := <-result:
		return u.user, u.err
	case <-time.After(lookupTimeout):
		return nil, trace.LimitExceeded("looking up the current host user exceeded timeout, try explicitly specifying host user if possible")
	}
}
