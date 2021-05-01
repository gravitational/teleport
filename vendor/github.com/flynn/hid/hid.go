// Package hid provides access to Human Interface Devices.
package hid

// DeviceInfo provides general information about a device.
type DeviceInfo struct {
	// Path contains a platform-specific device path which is used to identify the device.
	Path string

	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
	Manufacturer  string
	Product       string

	UsagePage uint16
	Usage     uint16

	InputReportLength  uint16
	OutputReportLength uint16
}

// A Device provides access to a HID device.
type Device interface {
	// Close closes the device and associated resources.
	Close()

	// Write writes an output report to device. The first byte must be the
	// report number to write, zero if the device does not use numbered reports.
	Write([]byte) error

	// ReadCh returns a channel that will be sent input reports from the device.
	// If the device uses numbered reports, the first byte will be the report
	// number.
	ReadCh() <-chan []byte

	// ReadError returns the read error, if any after the channel returned from
	// ReadCh has been closed.
	ReadError() error
}
