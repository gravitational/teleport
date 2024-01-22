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
	"time"

	"github.com/gravitational/trace"
)

const lookupTimeout = 10 * time.Second

type userGetter func() (*user.User, error)

type userResult struct {
	user *user.User
	err  error
}

// CurrentUser is just like [user.Current], except an error is returned
// if the user lookup exceeds 10 seconds. This is because if
// [user.Current] is called on a domain joined host, the user lookup
// will contact potentially multiple domain controllers which can hang.
func CurrentUser() (*user.User, error) {
	return currentUser(user.Current, lookupTimeout)
}

func currentUser(getUser userGetter, timeout time.Duration) (*user.User, error) {
	result := make(chan userResult, 1)
	go func() {
		u, err := getUser()
		result <- userResult{
			user: u,
			err:  err,
		}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case u := <-result:
		return u.user, trace.Wrap(u.err)
	case <-timer.C:
		return nil, trace.LimitExceeded(
			"unexpected host user lookup timeout, please explicitly specify the Teleport user with \"--user\" and the host user \"--login\" to skip host user lookup",
		)
	}
}
