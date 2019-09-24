// +build gcs

/*

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

package gcssessions

import (
	"context"
	"fmt"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
)

func TestGCS(t *testing.T) { check.TestingT(t) }

type GCSSuite struct {
	handler *Handler
	test.HandlerSuite
	gcsServer *fakestorage.Server
}

var _ = check.Suite(&GCSSuite{})

func (s *GCSSuite) SetUpSuite(c *check.C) {

	server := *fakestorage.NewServer([]fakestorage.Object{})
	s.gcsServer = &server

	ctx, cancelFunc := context.WithCancel(context.Background())

	utils.InitLoggerForTests()

	var err error
	s.HandlerSuite.Handler, err = NewHandler(ctx, cancelFunc, Config{
		Endpoint: server.URL(),
		Bucket:   fmt.Sprintf("teleport-test-%v", uuid.New()),
	}, server.Client())
	c.Assert(err, check.IsNil)
}

func (s *GCSSuite) TestUploadDownload(c *check.C) {
	s.UploadDownload(c)
}

func (s *GCSSuite) TestDownloadNotFound(c *check.C) {
	s.DownloadNotFound(c)
}

func (s *GCSSuite) TearDownSuite(c *check.C) {
	if s.gcsServer != nil {
		s.gcsServer.Stop()
	}
	if s.handler != nil {
		if err := s.handler.deleteBucket(); err != nil {
			c.Fatalf("Failed to delete bucket: %#v", trace.DebugReport(err))
		}
	}
}
