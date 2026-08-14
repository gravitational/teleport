package logging

import (
	"context"
	"log/slog"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common/packetcapture"
)

type normalLogger struct {
	logFile string
	capture *packetcapture.Capture
}

func newNormalLogger(logFile string, clock clockwork.Clock) *normalLogger {
	return &normalLogger{
		logFile: logFile,
		capture: packetcapture.NewCapture(clock),
	}
}

func (nl *normalLogger) Close() {
	err := nl.capture.SaveAsText(nl.logFile)
	if err != nil {
		slog.WarnContext(context.TODO(), "Failed to save packet trace to file.", "file", nl.logFile, "error", err)
	}
}

func (nl *normalLogger) LogPacket(direction packetcapture.Direction, packet protocol.Packet) {
	nl.capture.AddPacket(direction, packet.Payload())
}

func (nl *normalLogger) LogHeader(direction packetcapture.Direction, h protocol.PacketHeader) {}
