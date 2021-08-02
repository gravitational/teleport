/*
Copyright 2020 Gravitational, Inc.

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

package services

import (
	"time"

	"github.com/gravitational/teleport/api/types"
	"gopkg.in/check.v1"
)

type SemaphoreSuite struct{}

var _ = check.Suite(&SemaphoreSuite{})

func (s *SemaphoreSuite) TestAcquireSemaphoreRequest(c *check.C) {
	ok := types.AcquireSemaphoreRequest{
		SemaphoreKind: "foo",
		SemaphoreName: "bar",
		MaxLeases:     1,
		Expires:       time.Now(),
	}
	ok2 := ok
	c.Assert(ok.Check(), check.IsNil)
	c.Assert(ok2.Check(), check.IsNil)

	// Check that all the required fields have their
	// zero values rejected.
	bad := ok
	bad.SemaphoreKind = ""
	c.Assert(bad.Check(), check.NotNil)
	bad = ok
	bad.SemaphoreName = ""
	c.Assert(bad.Check(), check.NotNil)
	bad = ok
	bad.MaxLeases = 0
	c.Assert(bad.Check(), check.NotNil)
	bad = ok
	bad.Expires = time.Time{}
	c.Assert(bad.Check(), check.NotNil)

	// ensure that well formed acquire params can configure
	// a well formed semaphore.
	sem, err := ok.ConfigureSemaphore()
	c.Assert(err, check.IsNil)

	// verify acquisition works and semaphore state is
	// correctly updated.
	lease, err := sem.Acquire("sem-id", ok)
	c.Assert(err, check.IsNil)
	c.Assert(sem.Contains(*lease), check.Equals, true)

	// verify keepalive succeeds and correctly updates
	// semaphore expiry.
	newLease := *lease
	newLease.Expires = sem.Expiry().Add(time.Second)
	c.Assert(sem.KeepAlive(newLease), check.IsNil)
	c.Assert(sem.Expiry(), check.Equals, newLease.Expires)

	c.Assert(sem.Cancel(newLease), check.IsNil)
	c.Assert(sem.Contains(newLease), check.Equals, false)
}
