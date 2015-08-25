// backend represents interface for accessing configuration backend for storing ACL lists and other settings
package backend

import (
	"time"
)

// TODO(klizhentas) this is bloated. Split it into little backend interfaces
// Backend represents configuration backend implementation for Teleport
type Backend interface {
	GetKeys(path []string) ([]string, error)
	UpsertVal(path []string, key string, val []byte, ttl time.Duration) error
	GetVal(path []string, key string) ([]byte, error)
	DeleteKey(path []string, key string) error
	DeleteBucket(path []string, bkt string) error
}
