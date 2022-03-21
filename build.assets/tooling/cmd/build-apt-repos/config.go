package main

import (
	"flag"
	"os"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type Config struct {
	artifactPath    string
	bucketName      string
	localBucketPath string
	majorVersion    string
	releaseChannel  string
	logLevel        uint
	logJson         bool
}

// Parses and validates the provided flags, returning the parsed arguments in a struct.
func ParseFlags() (*Config, error) {
	config := &Config{
		artifactPath:    *flag.String("artifact-path", "/artifacts", "Path to the filesystem tree containing the *.deb files to add to the APT repos"),
		bucketName:      *flag.String("bucket", "", "The name of the S3 bucket where the repo should be synced to/from"),
		localBucketPath: *flag.String("local-bucket-path", "/bucket", "The local path where the bucket should be synced to"),
		majorVersion:    *flag.String("artifact-major-version", "", "The major version of the artifacts that will be added to the APT repos"),
		releaseChannel:  *flag.String("artifact-release-channel", "", "The release channel of the APT repos that the artifacts should be added to"),
		logLevel:        *flag.Uint("log-level", uint(logrus.InfoLevel), "Log level from 0 to 6, 6 being the most verbose"),
		logJson:         *flag.Bool("log-json", false, "True if the log entries should use JSON format, false for text logging"),
	}

	flag.Parse()
	if err := Check(config); err != nil {
		return nil, trace.Wrap(err, "failed to validate flags")
	}

	return config, nil
}

func Check(config *Config) error {
	if err := validateArtifactPath(config.artifactPath); err != nil {
		return trace.Wrap(err, "failed to validate the artifact path flag")
	}
	if err := validateBucketName(config.bucketName); err != nil {
		return trace.Wrap(err, "failed to validate the bucket name flag")
	}
	if err := validateLocalBucketPath(config.localBucketPath); err != nil {
		return trace.Wrap(err, "failed to validate the local bucket path flag")
	}
	if err := validateMajorVersion(config.majorVersion); err != nil {
		return trace.Wrap(err, "failed to validate the major version flag")
	}
	if err := validateReleaseChannel(config.releaseChannel); err != nil {
		return trace.Wrap(err, "failed to validate the release channel flag")
	}
	if err := validateLogLevel(config.logLevel); err != nil {
		return trace.Wrap(err, "failed to validate the log level flag")
	}

	return nil
}

func validateArtifactPath(value string) error {
	if value == "" {
		return trace.BadParameter("the artifact-path flag should not be empty")
	}

	if stat, err := os.Stat(value); os.IsNotExist(err) {
		return trace.BadParameter("the artifact-path %q does not exist", value)
	} else if !stat.IsDir() {
		return trace.BadParameter("the artifact-path %q is not a directory", value)
	}

	return nil
}

func validateBucketName(value string) error {
	if value == "" {
		return trace.BadParameter("the bucket flag should not be empty")
	}

	return nil
}

func validateLocalBucketPath(value string) error {
	if value == "" {
		return trace.BadParameter("the local-bucket-path flag should not be empty")
	}

	if stat, err := os.Stat(value); err == nil && !stat.IsDir() {
		return trace.BadParameter("the local bucket path points to a file instead of a directory")
	}

	return nil
}

func validateMajorVersion(value string) error {
	if value == "" {
		return trace.BadParameter("the artifact-major-version flag should not be empty")
	}

	// Can somebody validate that all major versions (even for dev tags/etc.) should follow this pattern?
	regex := `^v\d+$`
	matched, err := regexp.MatchString(regex, value)
	if err != nil {
		return trace.Wrap(err, "failed to validate the artifact major version flag via regex")
	}

	if !matched {
		return trace.BadParameter("the artifact major version flag does not match %s", regex)
	}

	return nil
}

func validateReleaseChannel(value string) error {
	if value == "" {
		return trace.BadParameter("the artifact-release-channel flag should not be empty")
	}

	// Not sure what other channels we'd want to support, but they should be listed here
	validReleaseChannels := []string{"stable"}

	for _, validReleaseChannel := range validReleaseChannels {
		if value == validReleaseChannel {
			return nil
		}
	}

	return trace.BadParameter("the release channel contains an invalid value. Valid values are: %s", strings.Join(validReleaseChannels, ","))
}

func validateLogLevel(value uint) error {
	if value > 6 {
		return trace.BadParameter("the log-level flag should be between 0 and 6")
	}

	return nil
}
