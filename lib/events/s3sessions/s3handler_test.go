// +build dynamodb

/*
Copyright 2018 Gravitational, Inc.

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

package s3sessions

import (
	"fmt"
	"testing"

	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
)

type S3Suite struct {
	handler *Handler
	test.HandlerSuite
}

var _ = fmt.Printf
var _ = check.Suite(&S3Suite{})

func TestS3Sessions(t *testing.T) { check.TestingT(t) }

// SetUpSuite configured the handler (S3 upload and downloader).
func (s *S3Suite) SetUpSuite(c *check.C) {
	var err error

	utils.InitLoggerForTests()

	// Create the S3 handler.
	s.handler, err = NewHandler(Config{
		Region: "us-west-1",
		Path:   "/test/",
		Bucket: fmt.Sprintf("teleport-test-%v", uuid.New()),
	})
	c.Assert(err, check.IsNil)

	// Set the handler suite (where the tests are defined).
	s.HandlerSuite.Handler = s.handler
}

// TearDownSuite removed any resources (buckets) created by the test suite.
func (s *S3Suite) TearDownSuite(c *check.C) {
	if s.handler != nil {
		if err := s.handler.deleteBucket(); err != nil {
			c.Fatalf("Failed to delete bucket: %#v", trace.DebugReport(err))
		}
	}
}

func (s *S3Suite) SetUpTest(c *check.C)    {}
func (s *S3Suite) TearDownTest(c *check.C) {}

func (s *S3Suite) TestUploadDownload(c *check.C) {
	s.UploadDownload(c)
}

func (s *S3Suite) TestDownloadNotFound(c *check.C) {
	s.DownloadNotFound(c)
}

// TestCancelUpload makes sure an upload can be canceled and the file is
// not uploaded.
func (s *S3Suite) TestCancelUpload(c *check.C) {
	s.CancelUpload(c)
}
