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

package s3sessions

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
)

func TestS3ThirdParty(t *testing.T) { check.TestingT(t) }

type S3ThirdPartySuite struct {
	backend gofakes3.Backend
	faker   *gofakes3.GoFakeS3
	server  *httptest.Server
	handler *Handler
	test.HandlerSuite
}

var _ = check.Suite(&S3ThirdPartySuite{})

func (s *S3ThirdPartySuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()

	//fakes3
	var timeSource gofakes3.TimeSource
	s.backend = s3mem.New(s3mem.WithTimeSource(timeSource))
	s.faker = gofakes3.New(s.backend, gofakes3.WithLogger(gofakes3.GlobalLog()))
	s.server = httptest.NewServer(s.faker.Server())

	var err error
	s.HandlerSuite.Handler, err = NewHandler(Config{
		Credentials:                 credentials.NewStaticCredentials("YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", ""),
		Region:                      "us-west-1",
		Path:                        "/test/",
		Bucket:                      fmt.Sprintf("teleport-test-%v", uuid.New()),
		Endpoint:                    s.server.URL,
		DisableServerSideEncryption: true,
	})
	c.Assert(err, check.IsNil)
}

func (s *S3ThirdPartySuite) TestUploadDownload(c *check.C) {
	s.UploadDownload(c)
}

func (s *S3ThirdPartySuite) TestDownloadNotFound(c *check.C) {
	s.DownloadNotFound(c)
}

func (s *S3ThirdPartySuite) TearDownSuite(c *check.C) {
	if s.handler != nil {
		if err := s.handler.deleteBucket(); err != nil {
			c.Fatalf("Failed to delete bucket: %#v", trace.DebugReport(err))
		}
	}
}
