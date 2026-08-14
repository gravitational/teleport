package storage

// CollectedDataDriftError is returned when collected data drifts in an invalid
// or disallowed manner, either in relation to previously-collected data or to
// the device profile.
type CollectedDataDriftError struct {
	message string
}

// NewCollectedDataDriftError returns a new [CollectedDataDriftError] instance.
func NewCollectedDataDriftError(msg string) *CollectedDataDriftError {
	return &CollectedDataDriftError{
		message: msg,
	}
}

// Error returns the textual representation of [CollectedDataDriftError].
func (e *CollectedDataDriftError) Error() string {
	return e.message
}

// Is returns `true` if `err` is a [CollectedDataDriftError].
// Meant to be used with [errors.Is].
func (e *CollectedDataDriftError) Is(target error) bool {
	_, ok := target.(*CollectedDataDriftError)
	return ok
}
