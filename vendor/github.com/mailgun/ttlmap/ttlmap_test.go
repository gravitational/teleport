package ttlmap

import (
	"testing"
	"time"

	"github.com/mailgun/timetools"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type TestSuite struct {
	timeProvider *timetools.FreezedTime
}

var _ = Suite(&TestSuite{})

func (s *TestSuite) SetUpTest(c *C) {
	start := time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)
	s.timeProvider = &timetools.FreezedTime{CurrentTime: start}
}

func (s *TestSuite) newMap(capacity int, opts ...TtlMapOption) *TtlMap {
	opts = append(opts, Clock(s.timeProvider))
	m, err := NewMap(capacity, opts...)
	if err != nil {
		panic(err)
	}
	return m
}

func (s *TestSuite) advanceSeconds(seconds int) {
	s.timeProvider.CurrentTime = s.timeProvider.CurrentTime.Add(time.Second * time.Duration(seconds))
}

func (s *TestSuite) TestValidation(c *C) {
	_, err := NewMapWithProvider(-1, s.timeProvider)
	c.Assert(err, Not(Equals), nil)

	_, err = NewMapWithProvider(0, s.timeProvider)
	c.Assert(err, Not(Equals), nil)

	_, err = NewMapWithProvider(1, nil)
	c.Assert(err, Not(Equals), nil)
}

func (s *TestSuite) TestWithRealTime(c *C) {
	m, err := NewMap(1)
	c.Assert(err, Equals, nil)
	c.Assert(m, Not(Equals), nil)
}

func (s *TestSuite) TestSetWrong(c *C) {
	m := s.newMap(1)

	err := m.Set("a", 1, -1)
	c.Assert(err, Not(Equals), nil)

	err = m.Set("a", 1, 0)
	c.Assert(err, Not(Equals), nil)

	_, err = m.Increment("a", 1, 0)
	c.Assert(err, Not(Equals), nil)

	_, err = m.Increment("a", 1, -1)
	c.Assert(err, Not(Equals), nil)
}

func (s *TestSuite) TestRemoveExpiredEmpty(c *C) {
	m := s.newMap(1)
	m.removeExpired(100)
}

func (s *TestSuite) TestRemoveLastUsedEmpty(c *C) {
	m := s.newMap(1)
	m.removeLastUsed(100)
}

func (s *TestSuite) TestGetSetExpire(c *C) {
	m := s.newMap(1)

	err := m.Set("a", 1, 1)
	c.Assert(err, Equals, nil)

	valI, exists := m.Get("a")
	c.Assert(exists, Equals, true)
	c.Assert(valI, Equals, 1)

	s.advanceSeconds(1)

	_, exists = m.Get("a")
	c.Assert(exists, Equals, false)
}

func (s *TestSuite) TestSetOverwrite(c *C) {
	m := s.newMap(1)

	err := m.Set("o", 1, 1)
	c.Assert(err, Equals, nil)

	valI, exists := m.Get("o")
	c.Assert(exists, Equals, true)
	c.Assert(valI, Equals, 1)

	err = m.Set("o", 2, 1)
	c.Assert(err, Equals, nil)

	valI, exists = m.Get("o")
	c.Assert(exists, Equals, true)
	c.Assert(valI, Equals, 2)
}

func (s *TestSuite) TestRemoveExpiredEdgeCase(c *C) {
	m := s.newMap(1)

	err := m.Set("a", 1, 1)
	c.Assert(err, Equals, nil)

	s.advanceSeconds(1)

	err = m.Set("b", 2, 1)
	c.Assert(err, Equals, nil)

	valI, exists := m.Get("a")
	c.Assert(exists, Equals, false)

	valI, exists = m.Get("b")
	c.Assert(exists, Equals, true)
	c.Assert(valI, Equals, 2)

	c.Assert(len(m.elements), Equals, 1)
	c.Assert(m.expiryTimes.Len(), Equals, 1)
	c.Assert(m.Len(), Equals, 1)
}

func (s *TestSuite) TestRemoveOutOfCapacity(c *C) {
	m := s.newMap(2)

	err := m.Set("a", 1, 5)
	c.Assert(err, Equals, nil)

	s.advanceSeconds(1)

	err = m.Set("b", 2, 6)
	c.Assert(err, Equals, nil)

	err = m.Set("c", 3, 10)
	c.Assert(err, Equals, nil)

	valI, exists := m.Get("a")
	c.Assert(exists, Equals, false)

	valI, exists = m.Get("b")
	c.Assert(exists, Equals, true)
	c.Assert(valI, Equals, 2)

	valI, exists = m.Get("c")
	c.Assert(exists, Equals, true)
	c.Assert(valI, Equals, 3)

	c.Assert(len(m.elements), Equals, 2)
	c.Assert(m.expiryTimes.Len(), Equals, 2)
	c.Assert(m.Len(), Equals, 2)
}

func (s *TestSuite) TestGetNotExists(c *C) {
	m := s.newMap(1)
	_, exists := m.Get("a")
	c.Assert(exists, Equals, false)
}

func (s *TestSuite) TestGetIntNotExists(c *C) {
	m := s.newMap(1)
	_, exists, err := m.GetInt("a")
	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, false)
}

func (s *TestSuite) TestGetInvalidType(c *C) {
	m := s.newMap(1)
	m.Set("a", "banana", 5)

	_, _, err := m.GetInt("a")
	c.Assert(err, Not(Equals), nil)

	_, err = m.Increment("a", 4, 1)
	c.Assert(err, Not(Equals), nil)
}

func (s *TestSuite) TestIncrementGetExpire(c *C) {
	m := s.newMap(1)

	m.Increment("a", 5, 1)
	val, exists, err := m.GetInt("a")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 5)

	s.advanceSeconds(1)

	m.Increment("a", 4, 1)
	val, exists, err = m.GetInt("a")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 4)
}

func (s *TestSuite) TestIncrementOverwrite(c *C) {
	m := s.newMap(1)

	m.Increment("a", 5, 1)
	val, exists, err := m.GetInt("a")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 5)

	m.Increment("a", 4, 1)
	val, exists, err = m.GetInt("a")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 9)
}

func (s *TestSuite) TestIncrementOutOfCapacity(c *C) {
	m := s.newMap(1)

	m.Increment("a", 5, 1)
	val, exists, err := m.GetInt("a")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 5)

	m.Increment("b", 4, 1)
	val, exists, err = m.GetInt("b")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 4)

	val, exists, err = m.GetInt("a")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, false)
}

func (s *TestSuite) TestIncrementRemovesExpired(c *C) {
	m := s.newMap(2)

	m.Increment("a", 1, 1)
	m.Increment("b", 2, 2)

	s.advanceSeconds(1)
	m.Increment("c", 3, 3)

	val, exists, err := m.GetInt("a")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, false)

	val, exists, err = m.GetInt("b")
	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 2)

	val, exists, err = m.GetInt("c")
	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 3)
}

func (s *TestSuite) TestIncrementRemovesLastUsed(c *C) {
	m := s.newMap(2)

	m.Increment("a", 1, 10)
	m.Increment("b", 2, 11)
	m.Increment("c", 3, 12)

	val, exists, err := m.GetInt("a")

	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, false)

	val, exists, err = m.GetInt("b")
	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)

	c.Assert(val, Equals, 2)

	val, exists, err = m.GetInt("c")
	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 3)
}

func (s *TestSuite) TestIncrementUpdatesTtl(c *C) {
	m := s.newMap(1)

	m.Increment("a", 1, 1)
	m.Increment("a", 1, 10)

	s.advanceSeconds(1)

	val, exists, err := m.GetInt("a")
	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 2)
}

func (s *TestSuite) TestUpdate(c *C) {
	m := s.newMap(1)

	m.Increment("a", 1, 1)
	m.Increment("a", 1, 10)

	s.advanceSeconds(1)

	val, exists, err := m.GetInt("a")
	c.Assert(err, Equals, nil)
	c.Assert(exists, Equals, true)
	c.Assert(val, Equals, 2)
}

func (s *TestSuite) TestCallOnExpire(c *C) {
	var called bool
	var key string
	var val interface{}
	m := s.newMap(1, CallOnExpire(func(k string, el interface{}) {
		called = true
		key = k
		val = el
	}))

	err := m.Set("a", 1, 1)
	c.Assert(err, Equals, nil)

	valI, exists := m.Get("a")
	c.Assert(exists, Equals, true)
	c.Assert(valI, Equals, 1)

	s.advanceSeconds(1)

	_, exists = m.Get("a")
	c.Assert(exists, Equals, false)
	c.Assert(called, Equals, true)
	c.Assert(key, Equals, "a")
	c.Assert(val, Equals, 1)
}
