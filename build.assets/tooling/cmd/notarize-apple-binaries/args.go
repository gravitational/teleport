/*
Copyright 2022 Gravitational, Inc.

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

package main

import (
	"flag"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type Args struct {
	*LoggerConfig
	*GonConfig
}

func NewArgs() *Args {
	args := &Args{}
	args.LoggerConfig = NewLoggerConfig()
	args.GonConfig = NewGonConfig()

	return args
}

func (a *Args) Check() error {
	err := a.LoggerConfig.Check()
	if err != nil {
		return trace.Wrap(err, "failed to validate the logger config")
	}

	err = a.GonConfig.Check()
	if err != nil {
		return trace.Wrap(err, "failed to validate the gon config")
	}

	return nil
}

func usage() {
	fmt.Printf("Usage: %s [OPTIONS] BINARIES...\n", flag.CommandLine.Name())
	fmt.Println()
	flag.PrintDefaults()
}

func parseArgs() (*Args, error) {
	flag.Usage = usage

	args := NewArgs()
	flag.Parse()

	// This needs to be called as soon as possible so that the logger can
	// be used when checking args
	args.LoggerConfig.setupLogger()

	err := args.Check()
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse all arguments")
	}

	logrus.Debugf("Successfully parsed args: %v", args)
	return args, nil
}
