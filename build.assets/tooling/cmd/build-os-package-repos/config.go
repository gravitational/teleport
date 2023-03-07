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
	"os"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

const StableChannelFlagValue string = "stable"

type LoggerConfig struct {
	logLevel uint
	logJSON  bool
}

func NewLoggerConfigWithFlagset(fs *flag.FlagSet) *LoggerConfig {
	lc := &LoggerConfig{}
	fs.UintVar(&lc.logLevel, "log-level", uint(logrus.InfoLevel), "Log level from 0 to 6, 6 being the most verbose")
	fs.BoolVar(&lc.logJSON, "log-json", false, "True if the log entries should use JSON format, false for text logging")

	return lc
}

func (lc *LoggerConfig) Check() error {
	if err := lc.validateLogLevel(); err != nil {
		return trace.Wrap(err, "failed to validate the log level flag")
	}

	return nil
}

func (lc *LoggerConfig) validateLogLevel() error {
	if lc.logLevel > 6 {
		return trace.BadParameter("the log-level flag should be between 0 and 6")
	}

	return nil
}

type S3Config struct {
	bucketName         string
	localBucketPath    string
	maxConcurrentSyncs int
}

func NewS3ConfigWithFlagset(fs *flag.FlagSet) *S3Config {
	s3c := &S3Config{}
	fs.StringVar(&s3c.bucketName, "bucket", "", "The name of the S3 bucket where the repo should be synced to/from")
	fs.StringVar(&s3c.localBucketPath, "local-bucket-path", "/bucket", "The local path where the bucket should be synced to")
	fs.IntVar(&s3c.maxConcurrentSyncs, "max-concurrent-syncs", 16, "The maximum number of S3 bucket syncs that may run in parallel (-1 for unlimited, 16 default)")

	return s3c
}

func (s3c *S3Config) Check() error {
	if err := s3c.validateBucketName(); err != nil {
		return trace.Wrap(err, "failed to validate the bucket name flag")
	}
	if err := s3c.validateLocalBucketPath(); err != nil {
		return trace.Wrap(err, "failed to validate the local bucket path flag")
	}
	if err := s3c.validateMaxConcurrentSyncs(); err != nil {
		return trace.Wrap(err, "failed to validate the max concurrent syncs flag")
	}

	return nil
}

func (s3c *S3Config) validateBucketName() error {
	if s3c.bucketName == "" {
		return trace.BadParameter("the bucket flag should not be empty")
	}

	return nil
}

func (s3c *S3Config) validateLocalBucketPath() error {
	if s3c.localBucketPath == "" {
		return trace.BadParameter("the local-bucket-path flag should not be empty")
	}

	if stat, err := os.Stat(s3c.localBucketPath); err == nil && !stat.IsDir() {
		return trace.BadParameter("the local bucket path points to a file instead of a directory")
	}

	return nil
}

func (s3c *S3Config) validateMaxConcurrentSyncs() error {
	if s3c.maxConcurrentSyncs < -1 {
		return trace.BadParameter("the max-concurrent-syncs flag must be greater than -1")
	}

	return nil
}

// This type is common to all other config types
type Config struct {
	*LoggerConfig
	*S3Config
	artifactPath   string
	printHelp      bool
	releaseChannel string
	versionChannel string
}

func NewConfigWithFlagset(fs *flag.FlagSet) *Config {
	c := &Config{}
	c.LoggerConfig = NewLoggerConfigWithFlagset(fs)
	c.S3Config = NewS3ConfigWithFlagset(fs)

	fs.StringVar(&c.artifactPath, "artifact-path", "/artifacts", "Path to the filesystem tree containing the *.deb or *.rpm files to add to the repos")
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "-h" || f.Name == "--help" {
			c.printHelp = true
		}
	})
	fs.StringVar(&c.releaseChannel, "release-channel", "", "The release channel of the repos that the artifacts should be added to")
	fs.StringVar(&c.versionChannel, "version-channel", "", "The version channel of the repos that the artifacts should be added to. Semver values will be truncated to major version. Examples: \"v1.2.3\" (truncated to \"v1\"), \"cloud\".")

	return c
}

func (c *Config) Check() error {
	if err := c.LoggerConfig.Check(); err != nil {
		return trace.Wrap(err, "failed to validate logger config")
	}

	if err := c.S3Config.Check(); err != nil {
		return trace.Wrap(err, "failed to validate S3 config")
	}

	if err := c.validateArtifactPath(); err != nil {
		return trace.Wrap(err, "failed to validate the artifact path flag")
	}
	if err := c.validateVersionChannel(); err != nil {
		return trace.Wrap(err, "failed to validate the version channel flag")
	}
	if err := c.validateReleaseChannel(); err != nil {
		return trace.Wrap(err, "failed to validate the release channel flag")
	}

	return nil
}

func (c *Config) validateArtifactPath() error {
	if c.artifactPath == "" {
		return trace.BadParameter("the artifact-path flag should not be empty")
	}

	if stat, err := os.Stat(c.artifactPath); os.IsNotExist(err) {
		return trace.BadParameter("the artifact-path %q does not exist", c.artifactPath)
	} else if !stat.IsDir() {
		return trace.BadParameter("the artifact-path %q is not a directory", c.artifactPath)
	}

	return nil
}

func (c *Config) validateVersionChannel() error {
	if c.versionChannel == "" {
		return trace.BadParameter("the version-channel flag should not be empty")
	}

	if semver.IsValid(c.versionChannel) {
		c.versionChannel = semver.Major(c.versionChannel)
	}

	return nil
}

func (c *Config) validateReleaseChannel() error {
	if c.releaseChannel == "" {
		return trace.BadParameter("the release-channel flag should not be empty")
	}

	// Not sure what other channels we'd want to support, but they should be listed here
	validReleaseChannels := []string{StableChannelFlagValue}

	for _, validReleaseChannel := range validReleaseChannels {
		if c.releaseChannel == validReleaseChannel {
			return nil
		}
	}

	return trace.BadParameter("the release channel contains an invalid value. Valid values are: %s", strings.Join(validReleaseChannels, ","))
}

// APT-specific config
type AptConfig struct {
	*Config
	aptlyPath string
}

func NewAptConfigWithFlagSet(fs *flag.FlagSet) (*AptConfig, error) {
	ac := &AptConfig{}
	ac.Config = NewConfigWithFlagset(fs)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get user's home directory path")
	}

	fs.StringVar(&ac.aptlyPath, "aptly-root-dir", homeDir, "The Aptly \"rootDir\" (see https://www.aptly.info/doc/configuration/ for details)")

	return ac, nil
}

func (ac *AptConfig) validateAptlyPath() error {
	if ac.aptlyPath == "" {
		return trace.BadParameter("the aptly-root-dir flag should not be empty")
	}

	return nil
}

func (ac *AptConfig) Check() error {
	if err := ac.Config.Check(); err != nil {
		return trace.Wrap(err, "failed to validate common config")
	}

	if err := ac.validateAptlyPath(); err != nil {
		return trace.Wrap(err, "failed to validate the aptly-root-dir path flag")
	}

	return nil
}

// YUM-specific config
type YumConfig struct {
	*Config
	cacheDir string
}

func NewYumConfigWithFlagSet(fs *flag.FlagSet) *YumConfig {
	yc := &YumConfig{}
	yc.Config = NewConfigWithFlagset(fs)

	fs.StringVar(&yc.cacheDir, "cache-dir", "/tmp/createrepo/cache", "The createrepo checksum caching directory (see https://linux.die.net/man/8/createrepo for details")

	return yc
}

func (yc *YumConfig) validateCacheDir() error {
	if yc.cacheDir == "" {
		return trace.BadParameter("the cache-dir flag should not be empty")
	}

	return nil
}

func (yc *YumConfig) Check() error {
	if err := yc.Config.Check(); err != nil {
		return trace.Wrap(err, "failed to validate common config")
	}

	if err := yc.validateCacheDir(); err != nil {
		return trace.Wrap(err, "failed to validate the cache-dir path flag")
	}

	return nil
}
