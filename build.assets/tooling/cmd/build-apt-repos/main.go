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
	if config.logJson {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{})
	}
	log.SetOutput(os.Stdout)
	log.SetLevel(log.Level(config.logLevel))
}
