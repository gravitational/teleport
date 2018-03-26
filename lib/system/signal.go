package system

/*
#include <signal.h>
int resetInterruptSignalHandler() {
	struct sigaction act;
	int result;
	if ((result = sigaction(SIGINT, 0, &act)) != 0) {
		return result;
	}
	if (act.sa_handler == SIG_IGN) {
		// Reset the handler for SIGINT to system default.
		// FIXME: Note, this will also overwrite runtime's signal handler
		signal(SIGINT, SIG_DFL);
	}
	return 0;
}
*/
import "C"
import log "github.com/sirupsen/logrus"

// ResetInterruptSignal will reset the handler for SIGINT back to the default
// handler. We need to do this because when sysvinit launches Teleport on some
// operating systems (like CentOS 6.8) it configures Teleport to ignore SIGINT
// signals. See the following for more details:
//
// http://garethrees.org/2015/08/07/ping/
// https://github.com/openssh/openssh-portable/commit/4e0f5e1ec9b6318ef251180dbca50eaa01f74536
func ResetInterruptSignalHandler() {
	_, err := C.resetInterruptSignalHandler()
	if err != nil {
		log.Warnf("Failed to reset interrupt signal handler: %v.", err)
	}
}
