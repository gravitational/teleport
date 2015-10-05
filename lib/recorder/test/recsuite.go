package test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/recorder"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestRecorder(t *testing.T) { TestingT(t) }

type RecorderSuite struct {
	R recorder.Recorder
}

func (s *RecorderSuite) Recorder(c *C) {

	w, err := s.R.GetChunkWriter("recs1")
	c.Assert(err, IsNil)

	c1 := recorder.Chunk{Data: []byte("chunk1")}
	c2 := recorder.Chunk{Delay: 3 * time.Millisecond, Data: []byte("chunk2")}
	c3 := recorder.Chunk{Delay: 5 * time.Millisecond, Data: []byte("chunk3")}

	c.Assert(w.WriteChunks([]recorder.Chunk{c1, c2}), IsNil)

	r1, err := s.R.GetChunkReader("recs1")
	c.Assert(err, IsNil)

	r2, err := s.R.GetChunkReader("recs1")
	c.Assert(err, IsNil)

	o, err := r1.ReadChunks(1, 2)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, []recorder.Chunk{c1})

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
