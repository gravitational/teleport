package utils

/*
#include <signal.h>
void resetInterruptSignalHandler() {
signal(SIGINT, SIG_DFL);
}
*/
import "C"

// ResetInterruptSignal will reset the handler for SIGINT back to the default
// handler. We need to do this because when sysvinit launches Teleport on some
// operating systems (like CentOS 6.8) it configures Teleport to ignore SIGINT
// signals. See the following for more details:
//
// http://garethrees.org/2015/08/07/ping/
// https://github.com/openssh/openssh-portable/commit/4e0f5e1ec9b6318ef251180dbca50eaa01f74536
func ResetInterruptSignalHandler() {
	C.resetInterruptSignalHandler()
}
