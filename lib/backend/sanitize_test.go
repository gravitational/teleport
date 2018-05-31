package backend

import (
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

func TestSanitizer(t *testing.T) { check.TestingT(t) }

type Suite struct {
}

var _ = check.Suite(&Suite{})
var _ = fmt.Printf

func (s *Suite) SetUpSuite(c *check.C) {
}

func (s *Suite) TearDownSuite(c *check.C) {
}

func (s *Suite) TearDownTest(c *check.C) {
}

func (s *Suite) SetUpTest(c *check.C) {
}

func (s *Suite) TestSanitizeBucket(c *check.C) {
	tests := []struct {
		inBucket []string
		inKey    string
		outError bool
	}{
		{
			inBucket: []string{"foo", "bar", "../../../etc/passwd"},
			inKey:    "",
			outError: true,
		},
		{
			inBucket: []string{},
			inKey:    "../../../etc/passwd",
			outError: true,
		},
		{
			inBucket: []string{"foo", "bar", "../../../etc/passwd"},
			inKey:    "../../../etc/passwd",
			outError: true,
		},
		{
			inBucket: []string{"foo", "bar"},
			inKey:    "baz-foo:bar.com",
			outError: false,
		},
	}

	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		safeBackend := NewSanitizer(&nopBackend{})

		if len(tt.inBucket) != 0 {
			_, err := safeBackend.GetKeys(tt.inBucket)
			c.Assert(err != nil, check.Equals, tt.outError, comment)

			err = safeBackend.CreateVal(tt.inBucket, tt.inKey, []byte{}, Forever)
			c.Assert(err != nil, check.Equals, tt.outError, comment)

			err = safeBackend.UpsertVal(tt.inBucket, tt.inKey, []byte{}, Forever)
			c.Assert(err != nil, check.Equals, tt.outError, comment)

			_, err = safeBackend.GetVal(tt.inBucket, tt.inKey)
			c.Assert(err != nil, check.Equals, tt.outError, comment)

			err = safeBackend.CompareAndSwapVal(tt.inBucket, tt.inKey, []byte{}, []byte{}, Forever)
			c.Assert(err != nil, check.Equals, tt.outError, comment)

			err = safeBackend.DeleteKey(tt.inBucket, tt.inKey)
			c.Assert(err != nil, check.Equals, tt.outError, comment)

			err = safeBackend.DeleteBucket(tt.inBucket, tt.inKey)
			c.Assert(err != nil, check.Equals, tt.outError, comment)
		}

		if tt.inKey != "" {
			err := safeBackend.AcquireLock(tt.inKey, Forever)
			c.Assert(err != nil, check.Equals, tt.outError, comment)

			err = safeBackend.ReleaseLock(tt.inKey)
			c.Assert(err != nil, check.Equals, tt.outError, comment)
		}
	}

}

type nopBackend struct {
}

func (n *nopBackend) GetKeys(bucket []string) ([]string, error) {
	return []string{"foo"}, nil
}

func (n *nopBackend) CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	return nil
}

func (n *nopBackend) UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	return nil
}

func (n *nopBackend) GetVal(path []string, key string) ([]byte, error) {
	return []byte("foo"), nil
}

func (n *nopBackend) CompareAndSwapVal(bucket []string, key string, val []byte, prevVal []byte, ttl time.Duration) error {
	return nil
}

func (n *nopBackend) DeleteKey(bucket []string, key string) error {
	return nil
}

func (n *nopBackend) DeleteBucket(path []string, bkt string) error {
	return nil
}

func (n *nopBackend) AcquireLock(token string, ttl time.Duration) error {
	return nil
}

func (n *nopBackend) ReleaseLock(token string) error {
	return nil
}

func (n *nopBackend) Close() error {
	return nil
}

func (n *nopBackend) Clock() clockwork.Clock {
	return clockwork.NewFakeClock()
}
