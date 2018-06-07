// +build !windows,!nacl,!plan9

package utils

import (
  "log/syslog"
  "os"

  log "github.com/sirupsen/logrus"
  logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

// SwitchLoggingtoSyslog tells the logger to send the output to syslog
func SwitchLoggingtoSyslog() {
	log.StandardLogger().SetHooks(make(log.LevelHooks))
	hook, err := logrusSyslog.NewSyslogHook("", "", syslog.LOG_WARNING, "")
	if err != nil {
		// syslog not available
		log.SetOutput(os.Stderr)
	} else {
		// ... and disable stderr:
		log.AddHook(hook)
		log.SetOutput(ioutil.Discard)
	}
}
