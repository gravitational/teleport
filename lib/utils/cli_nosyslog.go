// +build windows

package utils

import (
  "os"

  log "github.com/sirupsen/logrus"
)

func SwitchLoggingtoSyslog() {
  // syslog is not available on windows
  log.SetOutput(os.Stderr)
}
