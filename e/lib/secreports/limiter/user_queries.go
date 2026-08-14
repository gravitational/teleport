package limiter

import (
	"golang.org/x/sync/semaphore"
)

// NewUserQuery creates a new UserQuery Limiter.
func NewUserQuery(allowed int64) *UserQuery {
	return &UserQuery{
		sem: semaphore.NewWeighted(allowed),
	}
}

// UserQuery is a limiter that limits the number of concurrent async operations.
type UserQuery struct {
	sem *semaphore.Weighted
}

// Allow returns true if the operation is allowed to proceed.
func (a *UserQuery) Allow() bool {
	return a.sem.TryAcquire(1)
}

// Release releases the operation.
func (a *UserQuery) Release() {
	a.sem.Release(1)
}
