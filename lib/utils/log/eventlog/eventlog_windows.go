// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

// golang.org/x/sys
// Copyright 2009 The Go Authors.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google LLC nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// eventlog provides utilities based on golang.org/x/sys/windows/svc/eventlog. Unlike that package,
// this one can create event sources in custom logs, not just the Application log.
//
// https://learn.microsoft.com/en-us/windows/win32/eventlog/event-sources
// https://learn.microsoft.com/en-us/windows/win32/eventlog/eventlog-key
package eventlog

import (
	"errors"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/eventlog"
)

const (
	// EventID is a special event ID from Teleport's custom message file (msgfile.dll) which makes the
	// logged event display as just the text that was logged with it.
	//
	// Event Log requires the program to define all its messages upfront to make localization easier.
	// This special message lets us use Event Log more like syslog that accepts arbitrary messages.
	EventID = 10000
	// LogName is the name for a custom log that can be used with [Install].
	LogName = "Teleport"
)

// Install adds a registry entry for source in logName. Requires admin privileges to run. Once
// installed, source can be used in calls to [eventlog.Open] to write to the log.
//
// msgFile is a path to the message file DLL.
// https://learn.microsoft.com/en-us/windows/win32/eventlog/message-files
//
// If msgFile includes env variables, remember to pass true as useExpandKeyForMsgFile.
func Install(logName, source, msgFile string, useExpandKeyForMsgFile bool) error {
	return trace.Wrap(
		install(logName, source, msgFile, useExpandKeyForMsgFile, eventlog.Info|eventlog.Warning|eventlog.Error),
	)
}

// Remove removes the registry entry for source in logName. Requires admin privileges to run. It
// does not remove logs generated thus far, but Event Log will no longer be able to properly
// interpolate them – event data sent with log messages can still be manually extracted from them
// though using Get-WinEvent.
// It also does not remove the registry key for the custom log.
func Remove(logName, source string) error {
	return trace.Wrap(remove(logName, source))
}

func makeLogKeyName(logName string) string {
	return `SYSTEM\CurrentControlSet\Services\EventLog\` + logName
}

// install is a copy of [eventlog.Install] that makes it possible to create a new log under logName
// instead of adding a new event source under the Application log. Variable names are kept the same
// for the most part so that it's easy to compare this copy to the original after package updates.
//
// install modifies PC registry to allow logging with an event source src.
// It adds all required keys and values to the event log registry key.
// Install uses msgFile as the event message file. If useExpandKey is true,
// the event message file is installed as REG_EXPAND_SZ value,
// otherwise as REG_SZ. Use bitwise of log.Error, log.Warning and
// log.Info to specify events supported by the new event source.
func install(logName, source, msgFile string, useExpandKey bool, eventsSupported uint32) error {
	addKeyName := makeLogKeyName(logName)
	// registry.WRITE is needed for setting values on appKey and creating a sub key (sk).
	// registry.QUERY_VALUE is needed for reading values from appKey (MaxSize).
	// https://learn.microsoft.com/en-us/windows/win32/sysinfo/registry-key-security-and-access-rights
	appkey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, addKeyName, registry.WRITE|registry.QUERY_VALUE)
	if err != nil {
		return trace.Wrap(err, "creating registry key for log")
	}
	defer appkey.Close()

	// Set MaxSize for custom log if not already set. Without this, the default size for a custom log
	// is just 1 MB.
	//
	// Docs for registry values under an Eventlog key such as MaxSize:
	// https://learn.microsoft.com/en-us/windows/win32/eventlog/eventlog-key
	if _, _, err = appkey.GetIntegerValue("MaxSize"); err != nil {
		if !errors.Is(err, registry.ErrNotExist) {
			return trace.Wrap(err, "checking max size of log")
		}

		const defaultLogMaxSizeBytes = 20 * 1024 * 1024 // 20 MB
		if err := appkey.SetDWordValue("MaxSize", defaultLogMaxSizeBytes); err != nil {
			return trace.Wrap(err, "setting max size for log")
		}
	}

	sk, _, err := registry.CreateKey(appkey, source, registry.SET_VALUE)
	if err != nil {
		return trace.Wrap(err, "creating registry key for event source")
	}
	defer sk.Close()

	if err := sk.SetDWordValue("CustomSource", 1); err != nil {
		return trace.Wrap(err, "setting CustomSource")
	}
	if useExpandKey {
		err = sk.SetExpandStringValue("EventMessageFile", msgFile)
	} else {
		err = sk.SetStringValue("EventMessageFile", msgFile)
	}
	if err != nil {
		return trace.Wrap(err, "setting EventMessageFile")
	}
	if err := sk.SetDWordValue("TypesSupported", eventsSupported); err != nil {
		return trace.Wrap(err, "setting TypesSupported")
	}
	return nil
}

// remove is a copy of [eventlog.Remove] that is able to remove event sources from custom logs, not
// just the Application log.
//
// remove deletes all registry elements installed by the correspondent install.
func remove(logName, source string) error {
	addKeyName := makeLogKeyName(logName)
	appkey, err := registry.OpenKey(registry.LOCAL_MACHINE, addKeyName, registry.SET_VALUE)
	if err != nil {
		return trace.Wrap(err, "opening registry key for log")
	}
	defer appkey.Close()
	return trace.Wrap(registry.DeleteKey(appkey, source), "deleting registry key for event source")
}
