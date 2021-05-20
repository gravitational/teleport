// Package u2fhid implements the low-level FIDO U2F HID protocol.
package u2fhid

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/flynn/hid"
)

const (
	cmdPing  = 0x80 | 0x01
	cmdMsg   = 0x80 | 0x03
	cmdLock  = 0x80 | 0x04
	cmdInit  = 0x80 | 0x06
	cmdWink  = 0x80 | 0x08
	cmdSync  = 0x80 | 0x3c
	cmdError = 0x80 | 0x3f

	broadcastChannel = 0xffffffff

	capabilityWink = 1

	minMessageLen      = 7
	maxMessageLen      = 7609
	minInitResponseLen = 17

	responseTimeout = 3 * time.Second

	fidoUsagePage = 0xF1D0
	u2fUsage      = 1
)

var errorCodes = map[uint8]string{
	1: "invalid command",
	2: "invalid parameter",
	3: "invalid message length",
	4: "invalid message sequencing",
	5: "message timed out",
	6: "channel busy",
	7: "command requires channel lock",
	8: "sync command failed",
}

// Devices lists available HID devices that advertise the U2F HID protocol.
func Devices() ([]*hid.DeviceInfo, error) {
	devices, err := hid.Devices()
	if err != nil {
		return nil, err
	}

	res := make([]*hid.DeviceInfo, 0, len(devices))
	for _, d := range devices {
		if d.UsagePage == fidoUsagePage && d.Usage == u2fUsage {
			res = append(res, d)
		}
	}

	return res, nil
}

// Open initializes a communication channel with a U2F HID device.
func Open(info *hid.DeviceInfo) (*Device, error) {
	hidDev, err := info.Open()
	if err != nil {
		return nil, err
	}

	d := &Device{
		info:   info,
		device: hidDev,
		readCh: hidDev.ReadCh(),
	}

	if err := d.init(); err != nil {
		return nil, err
	}

	return d, nil
}

// A Device is used to communicate with a U2F HID device.
type Device struct {
	ProtocolVersion    uint8
	MajorDeviceVersion uint8
	MinorDeviceVersion uint8
	BuildDeviceVersion uint8

	// RawCapabilities is the raw capabilities byte provided by the device
	// during initialization.
	RawCapabilities uint8

	// CapabilityWink is true if the device advertised support for the wink
	// command during initilization. Even if this flag is true, the device may
	// not actually do anything if the command is called.
	CapabilityWink bool

	info    *hid.DeviceInfo
	device  hid.Device
	channel uint32

	mtx    sync.Mutex
	readCh <-chan []byte
	buf    []byte
}

func (d *Device) sendCommand(channel uint32, cmd byte, data []byte) error {
	if len(data) > maxMessageLen {
		return fmt.Errorf("u2fhid: message is too long")
	}

	// zero buffer
	for i := range d.buf {
		d.buf[i] = 0
	}

	binary.BigEndian.PutUint32(d.buf[1:], channel)
	d.buf[5] = cmd
	binary.BigEndian.PutUint16(d.buf[6:], uint16(len(data)))

	n := copy(d.buf[8:], data)
	data = data[n:]

	if err := d.device.Write(d.buf); err != nil {
		return err
	}

	var seq uint8
	for len(data) > 0 {
		// zero buffer
		for i := range d.buf {
			d.buf[i] = 0
		}

		binary.BigEndian.PutUint32(d.buf[1:], channel)
		d.buf[5] = seq
		seq++
		n := copy(d.buf[6:], data)
		data = data[n:]
		if err := d.device.Write(d.buf); err != nil {
			return err
		}
	}
	return nil
}

func (d *Device) readResponse(channel uint32, cmd byte) ([]byte, error) {
	timeout := time.After(responseTimeout)

	haveFirst := false
	var buf []byte
	var expected int

	for {
		select {
		case msg, ok := <-d.readCh:
			if len(msg) < minMessageLen {
				return nil, fmt.Errorf("u2fhid: message is too short, only received %d bytes", len(msg))
			}
			if !ok {
				return nil, fmt.Errorf("u2fhid: error reading response, device closed")
			}
			if channel != binary.BigEndian.Uint32(msg) {
				continue
			}

			if msg[4] == cmdError {
				errMsg, ok := errorCodes[msg[7]]
				if !ok {
					return nil, fmt.Errorf("u2fhid: received unknown error response %d", msg[7])
				}
				return nil, fmt.Errorf("u2fhid: received error from device: %s", errMsg)
			}

			if !haveFirst {
				if msg[4] != cmd {
					return nil, fmt.Errorf("u2fhid: error reading response, unexpected command %d, wanted %d", msg[4], cmd)
				}
				haveFirst = true
				expected = int(binary.BigEndian.Uint16(msg[5:]))
				buf = make([]byte, 0, expected)
				msg = msg[7:]
				if len(msg) > expected {
					msg = msg[:expected]
				}
				buf = append(buf, msg...)
			} else {
				if msg[4]&0x80 != 0 {
					return nil, fmt.Errorf("u2fhid: error reading response, unexpected command %d, wanted continuation", msg[4])
				}
				msg = msg[5:]
				if len(msg) > expected-len(buf) {
					msg = msg[:expected-len(buf)]
				}
				buf = append(buf, msg...)
			}
			if len(buf) >= expected {
				return buf, nil
			}
		case <-timeout:
			return nil, fmt.Errorf("u2fhid: error reading response, read timed out")
		}
	}
}

func (d *Device) init() error {
	d.buf = make([]byte, d.info.OutputReportLength+1)

	nonce := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	if err := d.sendCommand(broadcastChannel, cmdInit, nonce); err != nil {
		return err
	}

	for {
		res, err := d.readResponse(broadcastChannel, cmdInit)
		if err != nil {
			return err
		}
		if len(res) < minInitResponseLen {
			return fmt.Errorf("u2fhid: init response is short, wanted %d, got %d bytes", minInitResponseLen, len(res))
		}
		if !bytes.Equal(nonce, res[:8]) {
			// nonce doesn't match, this init reply isn't for us
			continue
		}
		d.channel = binary.BigEndian.Uint32(res[8:])

		d.ProtocolVersion = res[12]
		d.MajorDeviceVersion = res[13]
		d.MinorDeviceVersion = res[14]
		d.BuildDeviceVersion = res[15]
		d.RawCapabilities = res[16]
		d.CapabilityWink = d.RawCapabilities&capabilityWink != 0
		break
	}

	return nil
}

// Command sends a command and associated data to the device and returns the
// response.
func (d *Device) Command(cmd byte, data []byte) ([]byte, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if err := d.sendCommand(d.channel, cmd, data); err != nil {
		return nil, err
	}
	return d.readResponse(d.channel, cmd)
}

// Ping sends data to the device that should be echoed back verbatim.
func (d *Device) Ping(data []byte) ([]byte, error) {
	return d.Command(cmdPing, data)
}

// Wink performs a vendor-defined action to identify the device, like blinking
// an LED. It is not implemented correctly or at all on all devices.
func (d *Device) Wink() error {
	_, err := d.Command(cmdWink, nil)
	return err
}

// Message sends an encapsulated U2F protocol message to the device and returns
// the response.
func (d *Device) Message(data []byte) ([]byte, error) {
	return d.Command(cmdMsg, data)
}

// Close closes the device and frees associated resources.
func (d *Device) Close() {
	d.device.Close()
}
