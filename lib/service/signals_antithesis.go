//go:build antithesis && cgo

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
