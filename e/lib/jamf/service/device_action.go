package service

// DeviceAction is an action taken against devices during an inventory sync.
// Typically used to determine what to do with "missing" devices.
type DeviceAction int

const (
	// DeviceActionNoop is the "noop" action (aka no action).
	DeviceActionNoop DeviceAction = iota
	// DeviceActionDelete specifies for the device(s) in question to be deleted.
	DeviceActionDelete
)
