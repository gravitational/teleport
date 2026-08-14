package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/packetcapture"
)

// PacketLogger enables detailed logging of individual packets.
type PacketLogger interface {
	Close()
	LogPacket(direction packetcapture.Direction, p protocol.Packet)
	LogHeader(direction packetcapture.Direction, h protocol.PacketHeader)
}

const devMode = "DEV"
const normalMode = "NORMAL"
const silentMode = "SILENT"

func getLoggingDir() string {
	return os.Getenv("TELEPORT_UNSTABLE_ORACLE_LOGDIR")
}

func getLoggingMode() string {
	mode := os.Getenv("TELEPORT_UNSTABLE_ORACLE_LOGGER")

	if mode == "yes" {
		return normalMode
	}

	if mode == "dev" {
		return devMode
	}

	return silentMode
}

// NewPacketLogger creates a new PacketLogger instance. The logger chosen depends on the env variable.
// The default logger is silent and discards the logged information.
// A 'normal' logger logs raw packets to file using a simple text format. This one is suitable for running in restricted environments.
// A 'dev' logger applies further processing to packets: extracts debug data, converts the packets to .pcap format. It has external dependencies and is suited for dev work.
func NewPacketLogger(ctx context.Context, session *common.Session, log *slog.Logger) (PacketLogger, error) {
	mode := getLoggingMode()
	if mode == silentMode {
		return &nullLogger{}, nil
	}

	logDir := getLoggingDir()
	if logDir == "" {
		log.WarnContext(ctx, "No log directory set, disabling logging.")
		return &nullLogger{}, nil
	}

	proto := session.Database.GetProtocol()
	fileName := path.Join(logDir, fmt.Sprintf("%v-%s.log", proto, session.ID))

	log.InfoContext(ctx, "Logging packets to file", "file_name", fileName, "mode", mode)

	if mode == devMode {
		return newDevPacketLogger(fileName), nil
	}

	return newNormalLogger(fileName, clockwork.NewRealClock()), nil
}
