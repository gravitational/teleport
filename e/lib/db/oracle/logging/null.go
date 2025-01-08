package logging

import (
	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common/packetcapture"
)

type nullLogger struct{}

func (n nullLogger) Close() {}

func (n nullLogger) LogPacket(direction packetcapture.Direction, p protocol.Packet) {}

func (n nullLogger) LogHeader(direction packetcapture.Direction, h protocol.PacketHeader) {}
