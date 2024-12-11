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

package packettrace

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// Trace struct holds all packet details and offers methods to add and save packets.
type Trace struct {
	packets    []PacketEntry
	clock      clockwork.Clock
	runCommand func(name string, arg ...string) ([]byte, error)

	mx sync.Mutex
}

// NewTrace initializes a new Trace object.
func NewTrace(clock clockwork.Clock) *Trace {
	return &Trace{
		packets: make([]PacketEntry, 0),
		clock:   clock,
		runCommand: func(name string, arg ...string) ([]byte, error) {
			cmd := exec.Command(name, arg...)
			out, err := cmd.CombinedOutput()
			return out, trace.Wrap(err)
		},
	}
}

// AddPacket adds a packet to the trace based on the sender (client or server) and the payload.
// It automatically handles source and destination addresses based on the sender.
func (t *Trace) AddPacket(direction Direction, payload []byte) {
	// Record timestamp
	timestamp := t.clock.Now()

	// Create a new PacketEntry
	packet := PacketEntry{
		Direction: direction,
		Payload:   payload,
		Timestamp: timestamp,
	}

	// Add packet to the trace
	t.mx.Lock()
	t.packets = append(t.packets, packet)
	t.mx.Unlock()
}

type participant struct {
	addr      string
	direction Direction
}

func (t *Trace) saveOneLinkToPCAP(filename string, port int, sender, receiver participant, packets []PacketEntry) error {
	buffer := &bytes.Buffer{}
	for _, packet := range packets {
		// assign indicator or skip packet.
		indicator := ""
		if packet.Direction == sender.direction {
			indicator = "I"
		}
		if packet.Direction == receiver.direction {
			indicator = "O"
		}
		if indicator == "" {
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
	out, err := t.runCommand(text2pcapBin,
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

// SaveToPCAP saves the trace to a single merged pcap file.
func (t *Trace) SaveToPCAP(baseFilename string, port int) error {
	// Skip saving if filename is empty.
	if baseFilename == "" {
		return nil
	}

	t.mx.Lock() // Lock the mutex for the duration of the packet writing
	defer t.mx.Unlock()

	// we write to .pcap files:
	// client <-> teleport
	filename001 := baseFilename + ".001"
	clientPart := participant{addr: fakeClientAddr, direction: ClientToTeleport}
	teleportPart1 := participant{addr: fakeTeleportAddr, direction: TeleportToClient}
	err := t.saveOneLinkToPCAP(filename001, port, clientPart, teleportPart1, t.packets)
	if err != nil {
		return trace.Wrap(err, "error saving to PCAP")
	}
	defer os.Remove(filename001)

	// teleport <-> server
	filename002 := baseFilename + ".002"
	teleportPart2 := participant{addr: fakeTeleportAddr, direction: TeleportToServer}
	serverPart := participant{addr: fakeServerAddr, direction: ServerToTeleport}
	err = t.saveOneLinkToPCAP(filename002, port, teleportPart2, serverPart, t.packets)
	if err != nil {
		return trace.Wrap(err, "error saving to PCAP")
	}
	defer os.Remove(filename002)

	// merge two files
	out, err := t.runCommand(mergecapBin, "-w", baseFilename, filename001, filename002)
	if err != nil {
		return trace.Wrap(err, "error running mergecap (output: %v)", out)
	}
	return nil
}

func (t *Trace) SaveAsText(file string) error {
	// Skip saving if filename is empty.
	if file == "" {
		return nil
	}

	t.mx.Lock() // Lock the mutex for the duration of the packet writing
	defer t.mx.Unlock()

	var lines []string

	for _, packet := range t.packets {
		hexData := hex.Dump(packet.Payload)

		line := fmt.Sprintf("Timestamp: %v\nDirection: %v\n\n%s\n", packet.Timestamp.UTC(), packet.Direction, hexData)
		lines = append(lines, line)
	}

	result := []byte(strings.Join(lines, "\n"))

	return trace.Wrap(os.WriteFile(file, result, 0644))
}
