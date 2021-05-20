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

package labels

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type LabelSuite struct {
}

var _ = check.Suite(&LabelSuite{})

func TestLabels(t *testing.T) { check.TestingT(t) }

func (s *LabelSuite) TestSync(c *check.C) {
	// Create dynamic labels and sync right away.
	l, err := NewDynamic(context.Background(), &DynamicConfig{
		Labels: map[string]services.CommandLabel{
			"foo": &types.CommandLabelV2{
				Period:  services.NewDuration(1 * time.Second),
				Command: []string{"expr", "1", "+", "3"},
			},
		},
	})
	c.Assert(err, check.IsNil)
	l.Sync()

	// Check that the result contains the output of the command.
	c.Assert(l.Get()["foo"].GetResult(), check.Equals, "4")
}

func (s *LabelSuite) TestStart(c *check.C) {
	// Create dynamic labels and setup async update.
	l, err := NewDynamic(context.Background(), &DynamicConfig{
		Labels: map[string]services.CommandLabel{
			"foo": &types.CommandLabelV2{
				Period:  services.NewDuration(1 * time.Second),
				Command: []string{"expr", "1", "+", "3"},
			},
		},
	})
	c.Assert(err, check.IsNil)
	l.Start()

	// Wait a maximum of 5 seconds for dynamic labels to be updated.
	select {
	case <-time.Tick(50 * time.Millisecond):
		val, ok := l.Get()["foo"]
		c.Assert(ok, check.Equals, true)
		if val.GetResult() == "4" {
			break
		}
	case <-time.After(5 * time.Second):
		c.Fatalf("Timed out waiting for label to be updated.")
	}
}

// TestInvalidCommand makes sure that invalid commands return a error message.
func (s *LabelSuite) TestInvalidCommand(c *check.C) {
	// Create invalid labels and sync right away.
	l, err := NewDynamic(context.Background(), &DynamicConfig{
		Labels: map[string]services.CommandLabel{
			"foo": &types.CommandLabelV2{
				Period:  services.NewDuration(1 * time.Second),
				Command: []string{uuid.New()}},
		},
	})
	c.Assert(err, check.IsNil)
	l.Sync()

	// Check that the output contains that the command was not found.
	val, ok := l.Get()["foo"]
	c.Assert(ok, check.Equals, true)
	c.Assert(strings.Contains(val.GetResult(), "output:"), check.Equals, true)
}
