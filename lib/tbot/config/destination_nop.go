package config

import (
	"context"
	"github.com/gravitational/trace"
)

const DestinationNopType = "nop"

// DestinationNop on
type DestinationNop struct{}

func (dm *DestinationNop) CheckAndSetDefaults() error {
	return nil
}

func (dm *DestinationNop) Init(_ context.Context, subdirs []string) error {
	// Nothing to do.
	return nil
}

func (dm *DestinationNop) Verify(keys []string) error {
	// Nothing to do.
	return nil
}

func (dm *DestinationNop) Write(_ context.Context, name string, data []byte) error {
	// Nothing to do.
	return nil
}

func (dm *DestinationNop) Read(_ context.Context, name string) ([]byte, error) {
	// Nothing to do.
	return nil, trace.NotFound("reading from a nop destination results in no data")
}

func (dm *DestinationNop) String() string {
	return DestinationNopType
}

func (dm *DestinationNop) TryLock() (func() error, error) {
	// As this is purely in-memory, no locking behavior is required for the
	// Destination.
	return func() error {
		return nil
	}, nil
}

func (dm DestinationNop) MarshalYAML() (interface{}, error) {
	type raw DestinationNop
	return withTypeHeader(raw(dm), DestinationNopType)
}
