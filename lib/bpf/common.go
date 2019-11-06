// +build linux

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bpf

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentBPF,
})

// SessionContext contains all the information needed to track and emit
// events for a particular session. Most of this information is already within
// srv.ServerContext, unfortunately due to circular imports with lib/srv and
// lib/bpf, part of that structure is reproduced in SessionContext.
type SessionContext struct {
	// Namespace is the namespace within which this session occurs.
	Namespace string

	// SessionID is the UUID of the given session.
	SessionID string

	// ServerID is the UUID of the server this session is executing on.
	ServerID string

	// Login is the Unix login for this session.
	Login string

	// User is the Teleport user.
	User string

	// PID is the process ID of Teleport when it re-executes itself. This is
	// used by Telepor to find itself by cgroup.
	PID int

	// AuditLog is used to store events for a particular sessionl
	AuditLog events.IAuditLog
}

// attachProbe will attach a kprobe to the given function name.
func attachProbe(module *bcc.Module, eventName string, functionName string) error {
	kprobe, err := module.LoadKprobe(functionName)
	if err != nil {
		return trace.Wrap(err)
	}

	err = module.AttachKprobe(eventName, kprobe, -1)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// attachRetProbe will attach a kretprobe to the given function name.
func attachRetProbe(module *bcc.Module, eventName string, functionName string) error {
	kretprobe, err := module.LoadKprobe(functionName)
	if err != nil {
		return trace.Wrap(err)
	}

	err = module.AttachKretprobe(eventName, kretprobe, -1)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// openPerfBuffer will open a perf buffer for a particular module.
func openPerfBuffer(module *bcc.Module, perfMaps []*bcc.PerfMap, name string) (<-chan []byte, error) {
	var err error

	eventCh := make(chan []byte, 1024)
	table := bcc.NewTable(module.TableId(name), module)

	perfMap, err := bcc.InitPerfMap(table, eventCh)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	perfMap.Start()

	perfMaps = append(perfMaps, perfMap)

	return eventCh, nil
}

const (
	// commMax is the maximum length of a command from linux/sched.h.
	commMax = 16

	// pathMax is the maximum length of a path from linux/limits.h.
	pathMax = 255

	// argvMax is the maximum length of the args vector.
	argvMax = 128

	// eventArg is an exec event that holds the arguments to a function.
	eventArg = 0

	// eventRet holds the return value and other data about about an event.
	eventRet = 1
)
