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
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	supportedOSs := map[string][]string{
		"debian": { // See https://wiki.debian.org/DebianReleases#Production_Releases for details
			"stretch",  // 9
			"buster",   // 10
			"bullseye", // 11
			"bookwork", // 12
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
			"impish",   // 21.10 (EOL on 7/14/22)
			"jammy",    // 22.04 LTS
		},
	}

	config, err := ParseFlags()
	if err != nil {
		log.Fatal(err.Error())
	}

	setupLogger(config)
	log.Debugf("Starting tool with config: %v", config)

	art, err := NewAptRepoTool(config, supportedOSs)
	if err != nil {
		log.Fatal(err.Error())
	}

	err = art.Run()
	if err != nil {
		log.Fatal(err.Error())
	}
}

func setupLogger(config *Config) {
	if config.logJSON {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{})
	}
	log.SetOutput(os.Stdout)
	log.SetLevel(log.Level(config.logLevel))
}
