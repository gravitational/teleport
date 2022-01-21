package config

import (
	"github.com/gravitational/trace"
)

// Destination can persist renewable certificates.
type Destination interface {
	// Write stores data to the destination with the given name.
	Write(name string, data []byte) error

	// Read fetches data from the destination with a given name.
	Read(name string) ([]byte, error)
}

// DestinationMixin is a reusable struct for all config elements that accept a
// destination. Note that if embedded, DestinationMixin.CheckAndSetDefaults()
// must be called.
type DestinationMixin struct {
	Directory *DestinationDirectory `yaml:"directory,omitempty"`
	Memory    *DestinationMemory    `yaml:"memory,omitempty"`
}

type DestinationDefaults = func(*DestinationMixin) error

func (dm *DestinationMixin) CheckAndSetDefaults(applyDefaults DestinationDefaults) error {
	notNilCount := 0

	if dm.Directory != nil {
		if err := dm.Directory.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		notNilCount += 1
	}

	if dm.Memory != nil {
		if err := dm.Memory.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		notNilCount += 1
	}

	if notNilCount == 0 {
		// use defaults
		if err := applyDefaults(dm); err != nil {
			return trace.Wrap(err)
		}
	} else if notNilCount > 1 {
		return trace.BadParameter("only one destination backend may be specified at a time")
	}

	return nil
}

// GetDestination returns the first non-nil Destination set. Note that
// CheckAndSetDefaults() does attempt to ensure that only a single
// destination is set, though this may change at runtime.
func (dm *DestinationMixin) GetDestination() (Destination, error) {
	if dm.Directory != nil {
		return dm.Directory, nil
	}

	if dm.Memory != nil {
		return dm.Memory, nil
	}

	return nil, trace.BadParameter("no valid destination exists")
}
