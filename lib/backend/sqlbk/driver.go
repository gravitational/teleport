/*
Copyright 2022 Gravitational, Inc.

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

package sqlbk

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// The following errors are used as signals returned by driver implementations to
// a backend instance. It is important to not return trace errors such as
// trace.AlreadyExists and trace.NotFound from driver implementations because
// they have a specific meaning when returned from the backend. It is the
// responsibility of the backend to return the correct type of error, not the
// driver.
var (
	// ErrRetry is set as a transaction error when the transaction should be retried
	// due to serialization failure.
	ErrRetry = errors.New("retry")

	// ErrNotFound is returned by a transaction when a SQL query returns sql.ErrNoRows.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists is returned by a transaction when a SQL query returns a
	// unique constraint violation.
	ErrAlreadyExists = errors.New("already exists")
)

// Driver defines the interface implemented by specific SQL backend
// implementations such as postgres.
type Driver interface {
	// BackendName returns the name of the backend that created the driver.
	BackendName() string

	// Config returns the SQL backend configuration.
	Config() *Config

	// Open the database. The returned DB represents a database connection pool
	// referencing a specific database instance.
	Open(context.Context) (DB, error)
}

// DB defines an interface to a database instance backed by a connection pool.
type DB interface {
	io.Closer

	// Begin a read/write transaction. Canceling context will rollback the
	// transaction.
	Begin(context.Context) Tx

	// ReadOnly begins a read-only transaction. Canceling context will rollback
	// the transaction. Calling a mutating Tx method will result in a failed
	// transaction.
	ReadOnly(context.Context) Tx
}

// Tx defines a database transaction. A transaction can be in one of three
// states: committed, error, or active. New transactions begin in an active
// state until either Commit or Rollback is called or another method call
// places it in an error state. Calling any method other than Err after Commit
// is called is an undefined operation.
type Tx interface {
	// Err returns a transaction error. Calling other Tx methods has no effect
	// on the state of the transaction.
	Err() error

	// Commit the transaction. The same error returned from the Err method is
	// returned from Commit when the transaction is in an error state.
	Commit() error

	// Rollback the transaction with an error. The error passed to Rollback is
	// converted to a trace error and set as the transaction error returned from
	// Err. If the transaction is already in an error state, the error is
	// overridden by the error passed. Passing a nil error is considered a bug,
	// but the rollback will continue with a generated error if the transaction
	// is not already in an error state.
	Rollback(error) error

	// DeleteEvents created before expiryTime.
	DeleteEvents(expiryTime time.Time)

	// DeleteExpiredLeases removes leases whose expires column is not null and is
	// less than the current time.
	DeleteExpiredLeases()

	// DeleteItems not referencing an event or a valid lease.
	DeleteItems()

	// DeleteLease by key returning the backend item ID from the deleted lease.
	// Zero is returned when the delete fails.
	DeleteLease(key []byte) (id int64)

	// DeleteLeaseRange removes all leases inclusively between startKey
	// and endKey. It returns the set of backend items deleted. The returned
	// items include only Key and ID.
	DeleteLeaseRange(startKey, endKey []byte) []backend.Item

	// GetEvents returns an ordered set of events up to limit whose ID is
	// greater than fromEventID.
	GetEvents(fromEventID int64, limit int) Events

	// GetExpiredLeases returns all leases whose expires field is less than
	// or equal to the current time.
	GetExpiredLeases() []backend.Lease

	// GetItem by key. Nil is returned if the item has expired.
	GetItem(key []byte) *backend.Item

	// GetItemRange returns a set of backend items whose key is inclusively between
	// startKey and endKey. The returned items are ordered by key, will not exceed
	// limit, and does not include expired items.
	GetItemRange(startKey, endKey []byte, limit int) []backend.Item

	// GetItemValue returns an item's value by key if the item has not expired.
	GetItemValue(key []byte) []byte

	// GetLastEventID returns the most recent eventid. Zero is returned when the
	// event table is empty.
	GetLastEventID() int64

	// InsertEvent for backend item with evenType.
	InsertEvent(types.OpType, backend.Item)

	// InsertItem creates a new backend item ID, inserts the item, and returns the
	// new ID. The transaction will be set to an ErrRetry failed state if the ID
	// generated is already taken, which can happen when multiple transactions
	// are attempting to add the same item (the test suite's concurrent test
	// produces this scenario).
	InsertItem(item backend.Item) (id int64)

	// LeaseExists returns true if a lease exists for key that has not expired.
	LeaseExists(key []byte) bool

	// UpdateLease creates or updates a backend item.
	UpdateLease(backend.Item)

	// UpsertLease for backend item. The transaction is set to a NotFound error
	// state if the backend item does not exist.
	UpsertLease(backend.Item)
}

// Events is returned from the GetEvents Tx method.
type Events struct {
	LastID        int64           // ID of the most recent event in BackendEvents.
	Remaining     int             // Number of events whose ID is greater than LastID.
	BackendEvents []backend.Event // Set of backend events.
}
