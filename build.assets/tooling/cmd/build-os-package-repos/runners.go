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

	"github.com/gravitational/trace"
)

// Pattern from https://www.digitalocean.com/community/tutorials/how-to-use-the-flag-package-in-go
type Runner interface {
	Init([]string) error
	Run() error
	GetLoggerConfig() *LoggerConfig
	Name() string
	Info() string
}

// APT implementation
type AptRunner struct {
	flags        *flag.FlagSet
	config       *AptConfig
	supportedOSs map[string][]string
}

func NewAptRunner() (*AptRunner, error) {
	runner := &AptRunner{
		supportedOSs: map[string][]string{
			// When adding a new supportedOS, update the lib/web/scipts/node-join/install.sh script
			// Otherwise, it will keep using the binary installation instead of the deb repo.
			"debian": { // See https://wiki.debian.org/DebianReleases#Production_Releases for details
				"stretch",  // 9
				"buster",   // 10
				"bullseye", // 11
				"bookworm", // 12
				"trixie",   // 13
			},
			"ubuntu": { // See https://wiki.ubuntu.com/Releases for details
				"xenial",   // 16.04 LTS
				"yakkety",  // 16.10 (EOL)
				"zesty",    // 17.04 (EOL)
				"artful",   // 17.10 (EOL)
				"bionic",   // 18.04 LTS
				"cosmic",   // 18.10 (EOL)
				"disco",    // 19.04 (EOL)
				"eoan",     // 19.10 (EOL)
				"focal",    // 20.04 LTS
				"groovy",   // 20.10 (EOL)
				"hirsuite", // 21.04 (EOL)
				"impish",   // 21.10 (EOL)
				"jammy",    // 22.04 LTS
			},
		},
	}

	runner.flags = flag.NewFlagSet(runner.Name(), flag.ExitOnError)
	config, err := NewAptConfigWithFlagSet(runner.flags)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a new APT config instance")
	}

	runner.config = config

	return runner, nil
}

func (ar AptRunner) Init(args []string) error {
	err := ar.flags.Parse(args)
	if err != nil {
		return trace.Wrap(err, "failed to parse arguments")
	}

	err = ar.config.Check()
	if err != nil {
		return trace.Wrap(err, "failed to validate APT config arguments")
	}

	return nil
}

func (ar AptRunner) Run() error {
	if ar.config.printHelp {
		ar.flags.Usage()
		return nil
	}

	art, err := NewAptRepoTool(ar.config, ar.supportedOSs)
	if err != nil {
		return trace.Wrap(err, "failed to create a new APT repo tool instance")
	}

	err = art.Run()
	if err != nil {
		return trace.Wrap(err, "APT runner failed")
	}

	return nil
}

func (AptRunner) Name() string {
	return "apt"
}

func (AptRunner) Info() string {
	return "builds APT repos"
}

func (ar AptRunner) GetLoggerConfig() *LoggerConfig {
	return ar.config.LoggerConfig
}

// YUM implementation
type YumRunner struct {
	flags        *flag.FlagSet
	config       *YumConfig
	supportedOSs map[string][]string
}

func NewYumRunner() (*YumRunner, error) {
	runner := &YumRunner{
		supportedOSs: map[string][]string{
			// When adding a new supportedOS, update the lib/web/scipts/node-join/install.sh script
			// Otherwise, it will keep using the binary installation instead of the yum repo.
			"rhel": { // See https://access.redhat.com/articles/3078 for details
				"7",
				"8",
				"9",
			},
			"centos": { // See https://endoflife.date/centos for details
				"7",
				"8",
				"9",
			},
			// "$releasever" is a hot mess for Amazon Linux. No good documentation on this outside of just running
			//  a container or EC2 instance and manually checking $releasever values
			"amzn": {
				// "latest"	// 1, aka 2018.03.0.20201028.0
				"2", // 2, aka 2.0.20201111.0
				// "2022.0.20220531" // 2022 (new naming scheme, preview) aka 2022.0.20220531
			},
		},
	}

	runner.flags = flag.NewFlagSet(runner.Name(), flag.ExitOnError)
	runner.config = NewYumConfigWithFlagSet(runner.flags)

	return runner, nil
}

func (yr YumRunner) Init(args []string) error {
	err := yr.flags.Parse(args)
	if err != nil {
		return trace.Wrap(err, "failed to parse arguments")
	}

	err = yr.config.Check()
	if err != nil {
		return trace.Wrap(err, "failed to validate YUM config arguments")
	}

	return nil
}

func (yr YumRunner) Run() error {
	yrt, err := NewYumRepoTool(yr.config, yr.supportedOSs)
	if err != nil {
		return trace.Wrap(err, "failed to create a new YUM repo tool instance")
	}

	err = yrt.Run()
	if err != nil {
		return trace.Wrap(err, "YUM runner failed")
	}

	return nil
}

func (YumRunner) Name() string {
	return "yum"
}

func (YumRunner) Info() string {
	return "builds YUM repos"
}

func (yr YumRunner) GetLoggerConfig() *LoggerConfig {
	return yr.config.LoggerConfig
}
