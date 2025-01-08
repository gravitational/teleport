package logging

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common/packetcapture"
)

// devPacketLogger is a specialized logger interface for packets.
type devPacketLogger struct {
	logFile  *os.File
	pcapFile string

	logChannel chan logMessage
	cancelFunc context.CancelFunc
	closeCtx   context.Context
	startTime  time.Time
	capture    *packetcapture.Capture

	wg sync.WaitGroup
}

func replaceExtension(filename, newExt string) string {
	base := filename[:len(filename)-len(filepath.Ext(filename))]
	return base + newExt
}

func newDevPacketLogger(fileName string) *devPacketLogger {
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		slog.WarnContext(context.Background(), "Failed to open log file, logging to stderr instead.", "file", fileName, "error", err)
		file = os.Stderr
	}

	ctx, cancel := context.WithCancel(context.Background())

	l := &devPacketLogger{
		logFile:  file,
		pcapFile: replaceExtension(fileName, ".pcap"),

		logChannel: make(chan logMessage, 1000),
		closeCtx:   ctx,
		cancelFunc: cancel,
		startTime:  time.Now(),
		capture:    packetcapture.NewCapture(clockwork.NewRealClock()),
	}
	l.wg.Add(1)
	go l.processLogMessages()

	return l
}

// logMessage is a struct that holds the data to log.
type logMessage struct {
	tag       string
	packet    protocol.Packet
	header    *protocol.PacketHeader
	jsonData  string
	hexDump   string
	debugData string
	elapsedMs int64 // Elapsed time in milliseconds since logger creation
}

// LogPacket logs a message with the associated packet information.
func (l *devPacketLogger) LogPacket(direction packetcapture.Direction, packet protocol.Packet) {
	if l.closed() {
		return
	}

	data := map[string]any{
		"packet": packet,
		"header": packet.Header(),
	}

	if packet.Header().PacketFlags != 0 {
		data["header_flags"] = packet.Header().PacketFlags.ToString()
	}

	jsonData, _ := json.MarshalIndent(data, "", "  ") // ignore error for brevity
	hexDump := hex.Dump(packet.Payload())

	debugData := "(none)"

	if dp, ok := packet.(protocol.DebugPacket); ok {
		dd, _ := json.MarshalIndent(dp.DebugData(), "", "  ") // ignore error for brevity
		debugData = "\n" + string(dd) + "\n"
	}

	l.capture.AddPacket(direction, packet.Payload())

	l.logChannel <- logMessage{
		tag:       direction.String(),
		packet:    packet,
		jsonData:  string(jsonData),
		hexDump:   hexDump,
		debugData: debugData,
		elapsedMs: time.Since(l.startTime).Milliseconds(),
	}
}

func (l *devPacketLogger) LogHeader(direction packetcapture.Direction, header protocol.PacketHeader) {
	if l.closed() {
		return
	}

	jsonData, _ := json.MarshalIndent(header, "", "  ") // ignore error for brevity
	hexDump := hex.Dump(header.HeaderBytes)

	l.logChannel <- logMessage{
		tag:       direction.String(),
		header:    &header,
		jsonData:  string(jsonData),
		hexDump:   hexDump,
		elapsedMs: time.Since(l.startTime).Milliseconds(),
	}
}

// processLogMessages handles writing log messages to the file.
func (l *devPacketLogger) processLogMessages() {
	defer l.wg.Done()

	// process messages until context is closed
	loop := true
	for loop {
		select {
		case msg := <-l.logChannel:
			l.writeMessage(msg)
		case <-l.closeCtx.Done():
			loop = false
		}
	}

	// drain remaining messages
	for {
		select {
		case msg := <-l.logChannel:
			l.writeMessage(msg)
		default:
			return
		}
	}
}

func (l *devPacketLogger) writeMessage(msg logMessage) {
	if msg.packet != nil {
		l.writePacket(msg)
	}
	if msg.header != nil {
		l.writeHeader(msg)
	}
}

func (l *devPacketLogger) writePacket(msg logMessage) {
	logLine := fmt.Sprintf(`
-----------------------------------
ELAPSED TIME: %d ms
SOURCE: %s
TYPE: %v
GO TYPE: %T
SIZE: %d
DEBUG: %s
JSON: 
%s

HEX: 
%s
`,
		msg.elapsedMs, msg.tag, msg.packet.Type(), msg.packet, msg.packet.Size(), msg.debugData, msg.jsonData, msg.hexDump)

	if _, err := l.logFile.WriteString(logLine); err != nil {
		slog.ErrorContext(context.TODO(), "Error writing to log file", "error", err, "filename", l.logFile.Name())
	}
}

func (l *devPacketLogger) writeHeader(msg logMessage) {
	logLine := fmt.Sprintf(`
H-H-H-H-H-H-H-H-H-H-H-H-H-H-H-H-H-H
ELAPSED TIME: %d ms
SOURCE: %s
TYPE: %v
SIZE: %d
JSON: 
%s

HEX: 
%s
`,
		msg.elapsedMs, msg.tag, msg.header.PacketType, msg.header.PacketSize, msg.jsonData, msg.hexDump)

	if _, err := l.logFile.WriteString(logLine); err != nil {
		slog.ErrorContext(context.TODO(), "Error writing to log file", "error", err, "filename", l.logFile.Name())
	}
}

func (l *devPacketLogger) closed() bool {
	return l.closeCtx.Err() != nil
}

// Close closes the logger and flushes any remaining log messages.
func (l *devPacketLogger) Close() {
	if l.closed() {
		return
	}

	l.cancelFunc()

	l.wg.Wait()

	err := l.logFile.Close()
	if err != nil {
		slog.ErrorContext(context.TODO(), "Error closing log file", "error", err)
	}

	const oraclePort = 1521
	err = l.capture.SaveToPCAP(l.pcapFile, oraclePort)
	if err != nil {
		slog.ErrorContext(context.TODO(), "Error writing PCAP file", "error", err)
	}
}
