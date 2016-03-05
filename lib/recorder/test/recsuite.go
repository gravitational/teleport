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
package test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/recorder"

	. "gopkg.in/check.v1"
)

func TestRecorder(t *testing.T) { TestingT(t) }

type RecorderSuite struct {
	R recorder.Recorder
}

func (s *RecorderSuite) Recorder(c *C) {

	w, err := s.R.GetChunkWriter("recs1")
	c.Assert(err, IsNil)

	c1 := recorder.Chunk{Data: []byte("chunk1"), ServerID: "id1"}
	c2 := recorder.Chunk{Delay: 3 * time.Millisecond, Data: []byte("chunk2"), ServerID: "id2"}
	c3 := recorder.Chunk{Delay: 5 * time.Millisecond, Data: []byte("chunk3"), ServerID: "id3"}

	c.Assert(w.WriteChunks([]recorder.Chunk{c1, c2}), IsNil)

	r1, err := s.R.GetChunkReader("recs1")
	c.Assert(err, IsNil)

	r2, err := s.R.GetChunkReader("recs1")
	c.Assert(err, IsNil)

	o, err := r1.ReadChunks(1, 2)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, []recorder.Chunk{c1})

	count, err := r1.GetChunksCount()
	c.Assert(err, IsNil)
	c.Assert(count, Equals, uint64(2))

	o, err = r2.ReadChunks(1, 3)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, []recorder.Chunk{c1, c2})

	c.Assert(w.WriteChunks([]recorder.Chunk{c3}), IsNil)

	c.Assert(w.Close(), IsNil)

	o, err = r1.ReadChunks(3, 4)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, []recorder.Chunk{c3})

	o, err = r1.ReadChunks(4, 6)
	c.Assert(err, IsNil)
	c.Assert(len(o), Equals, 0)

	c.Assert(r1.Close(), IsNil)
	c.Assert(r2.Close(), IsNil)
}
