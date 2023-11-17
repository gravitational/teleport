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
			// When adding a new supportedOS, update the lib/web/scripts/node-join/install.sh script
			// Otherwise, it will keep using the binary installation instead of the deb repo.
			"debian": { // See https://wiki.debian.org/DebianReleases#Production_Releases for details
				"stretch",  // 9
				"buster",   // 10
				"bullseye", // 11
				"bookworm", // 12
				"trixie",   // 13
				"forky",    // 14
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
				"kinetic",  // 22.10 (EOL)
				"lunar",    // 23.04
				"mantic",   // 23.10
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
			// When adding a new supportedOS, update the lib/web/scripts/node-join/install.sh script
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
			// Note that it is important that we specify "$VERSION_ID" in our installation docs instead of
			// "$releasever" due to Amazon's weird $releasever naming scheme. Amazon Linux 2022 and onward
			// specify the release date in their naming scheme which means if we use $releasever anywhere
			// then we need to update our supported versions monthly.
			// $releasever can be checked with:
			// `python3 -c 'import dnf; print(dnf.dnf.Base().conf.substitutions);'`
			"amzn": {
				// "latest"	// 1, aka 2018.03.0.20201028.0
				"2",    // 2, aka 2.0.20201111.0
				"2022", // (new naming scheme, preview) aka 2022.0.20220531
				"2023", // 2023, which should match 2023.0.20230503 and similar releasever
			},
			// SUSE uses Zypper, which is compatible with YUM repos
			// SUSE Linux Enterprise Edition
			"sles": { // See https://www.suse.com/support/kb/doc/?id=000019587 for details
				"12",
				"15",
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
