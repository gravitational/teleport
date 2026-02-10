/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reexec

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// FileFD is a file descriptor passed down from a parent process when
// Teleport is re-executing itself.
type FileFD = uintptr

const (
	// ConfigFile is used to pass the reexec configuration payload
	// (including the command, logging, and session settings) from the parent process.
	ConfigFile FileFD = 3 + iota
	// LogFile is used to emit logs from the child process to the parent
	// process.
	LogFile
	// ContinueFile is used to communicate to the child process that
	// it can continue after the parent process assigns a cgroup to the
	// child process.
	ContinueFile
	// ReadyFile is used to communicate to the parent process that
	// the child has completed any setup operations that must occur before
	// the child is placed into its cgroup.
	ReadyFile
	// TerminateFile is used to communicate to the child process that
	// the interactive terminal should be killed as the client ended the
	// SSH session and without termination the terminal process will be assigned
	// to pid 1 and "live forever". Killing the shell should not prevent processes
	// preventing SIGHUP to be reassigned (ex. processes running with nohup).
	TerminateFile
	// FirstExtraFile is the first file descriptor that will be valid when
	// extra files are passed to child processes without a terminal.
	FirstExtraFile FileFD = TerminateFile + 1
)

// FileFDs for terminal based exec sessions.
const (
	// PTYFileDeprecated is a placeholder for the unused PTY file that
	// was passed to the child process. The PTY should only be used in the
	// the parent process but was left here for compatibility purposes.
	PTYFileDeprecated = FirstExtraFile + iota
	// TTYFile is a TTY the parent process passes to the child process.
	TTYFile
)

// FileFDs for non-terminal based exec sessions.
const (
	// StdinFile is used to capture the stdin stream of the shell (grandchild) process.
	StdinFile = FirstExtraFile + iota
	// StdoutFile is used to capture the stdout stream of the shell (grandchild) process.
	StdoutFile
	// StderrFile is used to capture the stderr stream of the shell (grandchild) process.
	StderrFile
)

// FileFDs for SFTP sessions.
const (
	// FileTransferOutFile is used to pass write transfer data to the sftp (grandchild) process.
	FileTransferOutFile = FirstExtraFile + iota
	// FileTransferInFile is used to pass read transfer data from the sftp (grandchild) process.
	FileTransferInFile
	// AuditInFile is used to pass audit events from the sftp (grandchild) process.
	AuditInFile
)

// FileFDs for networking sessions.
const (
	// ListenerFile is a unix datagram socket listener.
	ListenerFile = FirstExtraFile + iota
)

// FDName returns a file descriptor name.
func FDName(f FileFD) string {
	return fmt.Sprintf("/proc/self/fd/%d", f)
}

// InitLogger initializes slog using log pipe configuration from the parent.
func InitLogger(name string, cfg LogConfig) {
	logWriter := os.NewFile(LogFile, FDName(LogFile))
	if logWriter == nil {
		return
	}

	fields, err := logutils.ValidateFields(cfg.ExtraFields)
	if err != nil {
		return
	}

	switch cfg.Format {
	case "text", "":
		logger := slog.New(logutils.NewSlogTextHandler(logWriter, logutils.SlogTextHandlerConfig{
			Level:            cfg.Level,
			EnableColors:     cfg.EnableColors,
			ConfiguredFields: fields,
			Padding:          cfg.Padding,
		}))
		slog.SetDefault(logger.With(teleport.ComponentKey, name))
	case "json":
		logger := slog.New(logutils.NewSlogJSONHandler(logWriter, logutils.SlogJSONHandlerConfig{
			Level:            cfg.Level,
			ConfiguredFields: fields,
		}))
		slog.SetDefault(logger.With(teleport.ComponentKey, name))
	default:
		return
	}
}
