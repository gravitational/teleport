// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package vnet

/*
#include <arpa/inet.h>
#include <libproc.h>
#include <stdlib.h>
#include <sys/proc_info.h>

// vnet_find_tcp_owner scans every process for a TCP socket whose local
// (source) port == lport and foreign (destination) port == fport, both given
// in host byte order. It returns the owning PID, 0 if no socket matched, or -1
// if the initial process listing failed.
static pid_t vnet_find_tcp_owner(uint16_t lport, uint16_t fport) {
	int bufsize = proc_listpids(PROC_ALL_PIDS, 0, NULL, 0);
	if (bufsize <= 0) {
		return -1;
	}
	// Add headroom in case the set of PIDs grew since the sizing call above.
	int cap = bufsize + 16 * (int)sizeof(pid_t);
	pid_t *pids = (pid_t *)malloc(cap);
	if (pids == NULL) {
		return -1;
	}
	int n = proc_listpids(PROC_ALL_PIDS, 0, pids, cap);
	if (n <= 0) {
		free(pids);
		return -1;
	}
	int count = n / (int)sizeof(pid_t);

	pid_t found = 0;
	for (int i = 0; i < count && found == 0; i++) {
		pid_t pid = pids[i];
		if (pid <= 0) {
			continue;
		}

		int fdsize = proc_pidinfo(pid, PROC_PIDLISTFDS, 0, NULL, 0);
		if (fdsize <= 0) {
			continue;
		}
		struct proc_fdinfo *fds = (struct proc_fdinfo *)malloc(fdsize);
		if (fds == NULL) {
			continue;
		}
		int fn = proc_pidinfo(pid, PROC_PIDLISTFDS, 0, fds, fdsize);
		int fdcount = fn / (int)sizeof(struct proc_fdinfo);

		for (int j = 0; j < fdcount; j++) {
			if (fds[j].proc_fdtype != PROX_FDTYPE_SOCKET) {
				continue;
			}
			struct socket_fdinfo si;
			int r = proc_pidfdinfo(pid, fds[j].proc_fd, PROC_PIDFDSOCKETINFO,
			                       &si, sizeof(si));
			if (r < (int)sizeof(si)) {
				continue;
			}
			if (si.psi.soi_kind != SOCKINFO_TCP) {
				continue;
			}
			// insi_lport/insi_fport are stored in network byte order.
			struct in_sockinfo *ini = &si.psi.soi_proto.pri_tcp.tcpsi_ini;
			uint16_t sock_lport = ntohs((uint16_t)ini->insi_lport);
			uint16_t sock_fport = ntohs((uint16_t)ini->insi_fport);
			if (sock_lport == lport && sock_fport == fport) {
				found = pid;
				break;
			}
		}
		free(fds);
	}

	free(pids);
	return found;
}
*/
import "C"

import (
	"log/slog"
	"unsafe"

	"github.com/gravitational/trace"
)

// newProcessResolver returns the macOS process resolver backed by libproc.
func newProcessResolver(_ *slog.Logger) processResolver {
	return darwinProcessResolver{}
}

type darwinProcessResolver struct{}

func (darwinProcessResolver) resolveTCP(srcPort, dstPort uint16) (peerProcess, error) {
	pid := C.vnet_find_tcp_owner(C.uint16_t(srcPort), C.uint16_t(dstPort))
	switch {
	case pid < 0:
		return peerProcess{}, trace.Errorf("listing processes to resolve TCP connection owner")
	case pid == 0:
		return peerProcess{}, trace.NotFound(
			"no process owns TCP connection with source port %d and destination port %d",
			srcPort, dstPort)
	}

	// proc_pidpath may fail if the process exited between the socket scan and
	// now; in that case still return the PID we found.
	buf := make([]byte, C.PROC_PIDPATHINFO_MAXSIZE)
	n := C.proc_pidpath(C.int(pid), unsafe.Pointer(&buf[0]), C.uint32_t(len(buf)))
	if n <= 0 {
		return peerProcess{PID: int(pid)}, nil
	}
	return peerProcess{
		PID:     int(pid),
		ExePath: string(buf[:n]),
	}, nil
}
