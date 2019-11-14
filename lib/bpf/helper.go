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

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentBPF,
})

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
func openPerfBuffer(module *bcc.Module, perfMaps []*bcc.PerfMap, pageCount int, name string) (<-chan []byte, <-chan uint64, error) {
	var err error

	eventCh := make(chan []byte, chanSize)
	lostCh := make(chan uint64, chanSize)

	table := bcc.NewTable(module.TableId(name), module)

	perfMap, err := bcc.InitPerfMap(table, eventCh, lostCh, uint(pageCount))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	perfMap.Start()

	perfMaps = append(perfMaps, perfMap)

	return eventCh, lostCh, nil
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

	// chanSize is the size of the event and lost event channels.
	chanSize = 1024
)
