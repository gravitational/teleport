package minheap

import (
	. "launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type MinHeapSuite struct{}

var _ = Suite(&MinHeapSuite{})

func toEl(i int) interface{} {
	return &i
}

func fromEl(i interface{}) int {
	return *(i.(*int))
}

func (s *MinHeapSuite) TestPeek(c *C) {
	mh := NewMinHeap()

	el := &Element{
		Value:    toEl(1),
		Priority: 5,
	}

	mh.PushEl(el)
	c.Assert(fromEl(mh.PeekEl().Value), Equals, 1)
	c.Assert(mh.Len(), Equals, 1)

	el = &Element{
		Value:    toEl(2),
		Priority: 1,
	}
	mh.PushEl(el)
	c.Assert(mh.Len(), Equals, 2)
	c.Assert(fromEl(mh.PeekEl().Value), Equals, 2)
	c.Assert(fromEl(mh.PeekEl().Value), Equals, 2)
	c.Assert(mh.Len(), Equals, 2)

	el = mh.PopEl()

	c.Assert(fromEl(el.Value), Equals, 2)
	c.Assert(mh.Len(), Equals, 1)
	c.Assert(fromEl(mh.PeekEl().Value), Equals, 1)

	mh.PopEl()
	c.Assert(mh.Len(), Equals, 0)
}

func (s *MinHeapSuite) TestUpdate(c *C) {
	mh := NewMinHeap()
	x := &Element{
		Value:    toEl(1),
		Priority: 4,
	}
	y := &Element{
		Value:    toEl(2),
		Priority: 3,
	}
	z := &Element{
		Value:    toEl(3),
		Priority: 8,
	}
	mh.PushEl(x)
	mh.PushEl(y)
	mh.PushEl(z)
	c.Assert(fromEl(mh.PeekEl().Value), Equals, 2)

	mh.UpdateEl(z, 1)
	c.Assert(fromEl(mh.PeekEl().Value), Equals, 3)

	mh.UpdateEl(x, 0)
	c.Assert(fromEl(mh.PeekEl().Value), Equals, 1)
}
