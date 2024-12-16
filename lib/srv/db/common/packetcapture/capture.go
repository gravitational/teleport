// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package packetcapture provides utilities for saving application-layer packets to file in either plain text or PCAP formats.
// The PCAP functionality depends on the external utilities from Wireshark (text2pcap, mergecap) and is expected to be used in dev/debugging contexts only.
package packetcapture

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Direction represents the link and participants.
type Direction int

const (
	ClientToTeleport Direction = iota
	ServerToTeleport
	TeleportToClient
	TeleportToServer
)

const fakeClientAddr = "1.1.1.1"
const fakeTeleportAddr = "2.2.2.2"
const fakeServerAddr = "3.3.3.3"

const mergecapBin = "mergecap"
const text2pcapBin = "text2pcap"

func (p Direction) String() string {
	switch p {
	case ClientToTeleport:
		return "Client->Teleport"
	case ServerToTeleport:
		return "Server->Teleport"
	case TeleportToClient:
		return "Teleport->Client"
	case TeleportToServer:
		return "Teleport->Server"
	default:
		return fmt.Sprintf("Unknown(%d)", p)
	}
}

// PacketEntry holds the details of each packet to be recorded.
type PacketEntry struct {
	Direction Direction
	Payload   []byte
	Timestamp time.Time
}

// Capture struct holds all packet details and offers methods to add and save packets.
type Capture struct {
	packets []PacketEntry
	clock   clockwork.Clock

	// runCommand runs a specific command and returns combined output.
	runCommand func(name string, arg ...string) ([]byte, error)

	mu sync.Mutex
}

// NewCapture initializes a new Capture object.
func NewCapture(clock clockwork.Clock) *Capture {
	return &Capture{
		packets: make([]PacketEntry, 0),
		clock:   clock,
		runCommand: func(name string, arg ...string) ([]byte, error) {
			cmd := exec.Command(name, arg...)
			out, err := cmd.CombinedOutput()
			return out, trace.Wrap(err)
		},
	}
}

// AddPacket records the packet in the given direction and payload.
func (c *Capture) AddPacket(direction Direction, payload []byte) {
	// Record timestamp
	timestamp := c.clock.Now()

	// Create a new PacketEntry
	packet := PacketEntry{
		Direction: direction,
		Payload:   payload,
		Timestamp: timestamp,
	}

	c.mu.Lock()
	c.packets = append(c.packets, packet)
	c.mu.Unlock()
}

type participant struct {
	addr      string
	direction Direction
}

func (c *Capture) saveOneLinkToPCAP(filename string, port int, sender, receiver participant, packets []PacketEntry) error {
	buffer := &bytes.Buffer{}
	for _, packet := range packets {
		// assign indicator or skip packet.
		var indicator string
		switch packet.Direction {
		case sender.direction:
			indicator = "I"
		case receiver.direction:
			indicator = "O"
		default:
			continue
		}

		// Write the timestamp and sender indicator
		if _, err := fmt.Fprintf(buffer, "%s %s\n", indicator, packet.Timestamp.UTC().Format(time.RFC3339Nano)); err != nil {
			return trace.Wrap(err)
		}

		// Write the packet data using hex.Dump and add a newline after each packet
		hexData := hex.Dump(packet.Payload)
		if _, err := buffer.Write([]byte(hexData + "\n")); err != nil {
			return trace.Wrap(err)
		}
	}

	filenameHex := filename + ".hex"

	err := os.WriteFile(filenameHex, buffer.Bytes(), 0600)
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.Remove(filenameHex)

	// Invoke text2pcap to convert the hex dump to PCAP format
	addrPair := fmt.Sprintf("%v,%v", sender.addr, receiver.addr)
	out, err := c.runCommand(text2pcapBin,
		// enable inbound/outbound markers
		"-D",
		// configure timestamp format
		"-t", "%Y-%m-%dT%H:%M:%S.%fZ",
		// ethernet capture encapsulation type
		"-l", "1",
		// specify IPv4 addr pair
		"-4", addrPair,
		// inbound/outbound ports; also used to hint at packet type; might be worth using "-P <dissector>" instead.
		"-T", fmt.Sprintf("%v,%v", port, port),
		filenameHex,
		filename)
	if err != nil {
		return trace.Wrap(err, "error running text2pcap (output: %v)", out)
	}

	return nil
}

// SaveToPCAP saves the captured packets to a single pcap file. Note: this will run `text2pcap` and `mergecap` programs.
func (c *Capture) SaveToPCAP(baseFilename string, port int) error {
	// Skip saving if filename is empty.
	if baseFilename == "" {
		return nil
	}

	c.mu.Lock() // Lock the mutex for the duration of the packet writing
	defer c.mu.Unlock()

	// we write to .pcap files:
	// client <-> teleport
	filename001 := baseFilename + ".001"
	clientPart := participant{addr: fakeClientAddr, direction: ClientToTeleport}
	teleportPart1 := participant{addr: fakeTeleportAddr, direction: TeleportToClient}
	err := c.saveOneLinkToPCAP(filename001, port, clientPart, teleportPart1, c.packets)
	if err != nil {
		return trace.Wrap(err, "error saving to PCAP")
	}
	defer os.Remove(filename001)

	// teleport <-> server
	filename002 := baseFilename + ".002"
	teleportPart2 := participant{addr: fakeTeleportAddr, direction: TeleportToServer}
	serverPart := participant{addr: fakeServerAddr, direction: ServerToTeleport}
	err = c.saveOneLinkToPCAP(filename002, port, teleportPart2, serverPart, c.packets)
	if err != nil {
		return trace.Wrap(err, "error saving to PCAP")
	}
	defer os.Remove(filename002)

	// merge two files
	out, err := c.runCommand(mergecapBin, "-w", baseFilename, filename001, filename002)
	if err != nil {
		return trace.Wrap(err, "error running mergecap (output: %v)", out)
	}
	return nil
}

func (c *Capture) WriteTo(w io.Writer) (int64, error) {
	c.mu.Lock() // Lock the mutex for the duration of the packet writing
	defer c.mu.Unlock()

	var total int64
	for _, packet := range c.packets {
		hexData := hex.Dump(packet.Payload)

		count, err := fmt.Fprintf(w, "Timestamp: %v\nDirection: %v\n\n%s\n\n", packet.Timestamp.UTC(), packet.Direction, hexData)
		total += int64(count)
		if err != nil {
			return total, trace.Wrap(err)
		}
	}
	return total, nil
}

// SaveAsText saves the capture using plain text format without any external dependencies.
func (c *Capture) SaveAsText(file string) error {
	// Skip saving if filename is empty.
	if file == "" {
		return nil
	}

	handle, err := os.Create(file)
	if err != nil {
		return trace.Wrap(err)
	}
	defer handle.Close()

	_, err = c.WriteTo(handle)
	return trace.Wrap(err)
}
