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

//go:build antithesis && cgo && linux

package service

/*
#include <signal.h>
#include <unistd.h>
#include <time.h>
#include <stdio.h>

static void stamp(const char *what, int s) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	char buf[96];
	int n = snprintf(buf, sizeof buf, "earlysig: %s sig=%d %ld.%09ld\n",
	                 what, s, (long)ts.tv_sec, ts.tv_nsec);
	write(2, buf, n);
}

static void h(int s) { stamp("caught", s); _exit(128+s); }

__attribute__((constructor)) static void install(void) {
	signal(SIGTERM, h);
	signal(SIGINT,  h);
	signal(SIGHUP,  h);
	stamp("installed", -1);
}
*/
import "C"
