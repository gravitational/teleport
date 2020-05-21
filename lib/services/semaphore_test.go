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
	"fmt"
	"time"

	//"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

type SemaphoreSuite struct {
}

var _ = Suite(&SemaphoreSuite{})
var _ = fmt.Printf

func (s *SemaphoreSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *SemaphoreSuite) TestAcquireSemaphoreParams(c *C) {
	ok := AcquireSemaphore{
		SemaphoreKind: "foo",
		SemaphoreName: "bar",
		MaxLeases:     1,
		Expires:       time.Now(),
	}
	ok2 := ok
	c.Assert(ok.CheckAndSetDefaults(), IsNil)
	c.Assert(ok2.CheckAndSetDefaults(), IsNil)

	// LeaseID must be randomly generated if not set
	c.Assert(ok.LeaseID, Not(Equals), ok2.LeaseID)

	// Check that all the required fields have their
	// zero values rejected.
	bad := ok
	bad.SemaphoreKind = ""
	c.Assert(bad.CheckAndSetDefaults(), NotNil)
	bad = ok
	bad.SemaphoreName = ""
	c.Assert(bad.CheckAndSetDefaults(), NotNil)
	bad = ok
	bad.MaxLeases = 0
	c.Assert(bad.CheckAndSetDefaults(), NotNil)
	bad = ok
	bad.Expires = time.Time{}
	c.Assert(bad.CheckAndSetDefaults(), NotNil)

	// ensure that well formed acquire params can configure
	// a well formed semaphore.
	sem, err := ok.ConfigureSemaphore()
	c.Assert(err, IsNil)

	// verify acquisition works and semaphore state is
	// correctly updated.
	lease, err := sem.Acquire(ok)
	c.Assert(err, IsNil)
	c.Assert(sem.Contains(*lease), Equals, true)

	// verify keepalive succeeds and correctly updates
	// semaphore expiry.
	newLease := *lease
	newLease.Expires = sem.Expiry().Add(time.Second)
	c.Assert(sem.KeepAlive(newLease), IsNil)
	c.Assert(sem.Expiry(), Equals, newLease.Expires)

	c.Assert(sem.Cancel(newLease), IsNil)
	c.Assert(sem.Contains(newLease), Equals, false)
}
