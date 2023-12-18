/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"debug/macho"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const binaryArgName string = "binaries"

// Default values for flags.
// TODO(camh): Remove when all call sites pass the correct values.
const (
	DeveloperIdentity string = "0FFD3E3413AB4C599C53FBB1D8CA690915E33D83"
	BundleID          string = "com.gravitational.teleport"
)

type Config struct {
	LogLevel      string
	LogJSON       bool
	AppleUsername string
	ApplePassword string
	DeveloperID   string
	BundleID      string
	BinaryPaths   []string
}

func main() {
	var config Config
	kingpin.Flag("log-level", "Output logging level").Default(logrus.InfoLevel.String()).EnumVar(&config.LogLevel, getLogLevelStrings()...)
	kingpin.Flag("log-json", "Enable JSON logging").Default(fmt.Sprintf("%v", false)).BoolVar(&config.LogJSON)
	kingpin.Flag("apple-username", "Apple Connect username used for notarization").Required().Envar("APPLE_USERNAME").StringVar(&config.AppleUsername)
	kingpin.Flag("apple-password", "Apple Connect password used for notarization").Required().Envar("APPLE_PASSWORD").StringVar(&config.ApplePassword)
	kingpin.Flag("developer-id", "Key ID for signing binaries").Default(DeveloperIdentity).StringVar(&config.DeveloperID)
	kingpin.Flag("bundle-id", "Bundle ID of application").Default(BundleID).StringVar(&config.BundleID)
	kingpin.Arg(binaryArgName, "Path to Apple binaries for signing and notarization").Required().Action(binaryArgValidatiorAction).ExistingFilesVar(&config.BinaryPaths)
	kingpin.Parse()

	err := run(&config)

	if err != nil {
		logrus.Fatal(err.Error())
	}
}

func getLogLevelStrings() []string {
	logLevelStrings := make([]string, 0, len(logrus.AllLevels))
	for _, level := range logrus.AllLevels {
		logLevelStrings = append(logLevelStrings, level.String())
	}

	return logLevelStrings
}

func binaryArgValidatiorAction(pc *kingpin.ParseContext) error {
	for _, element := range pc.Elements {
		if clause, ok := element.Clause.(*kingpin.ArgClause); !ok || clause.Model().Name != binaryArgName {
			continue
		}

		binaryPath := *element.Value
		err := verifyFileIsAppleBinary(binaryPath)
		if err != nil {
			return trace.Wrap(err, "failed to verify that %q is a valid Apple binary for signing", binaryPath)
		}
	}
	return nil
}

// Returns an error if the file is not a valid Apple binary
// Effectively does `file $BINARY | grep -ic 'mach-o'`
func verifyFileIsAppleBinary(filePath string) error {
	// First check to see if the binary is a typical mach-o binary.
	// If it's not, it could still be a multiarch "fat" mach-o binary,
	// so we try that next. If both fail then the file is not an Apple
	// binary.
	fileHandle, err := macho.Open(filePath)
	if err != nil {
		fatFileHandle, err := macho.OpenFat(filePath)
		if err != nil {
			return trace.Wrap(err, "the provided file %q is neither a normal or multiarch mach-o binary.", filePath)
		}

		err = fatFileHandle.Close()
		if err != nil {
			return trace.Wrap(err, "identified %q as a multiarch mach-o binary but failed to close the file handle", filePath)
		}
	} else {
		err := fileHandle.Close()
		if err != nil {
			return trace.Wrap(err, "identified %q as a mach-o binary but failed to close the file handle", filePath)
		}
	}

	return nil
}

func run(config *Config) error {
	// This needs to be called as soon as possible so that the logger can
	// be used when checking args
	parsedLogLevel, err := logrus.ParseLevel(config.LogLevel)
	if err != nil {
		// This should never be hit if kingpin is configured correctly
		return trace.Wrap(err, "failed to parse logrus log level")
	}
	NewLoggerConfig(parsedLogLevel, config.LogJSON).setupLogger()

	err = NewGonWrapper(config.AppleUsername, config.ApplePassword, config.DeveloperID, config.BundleID, config.BinaryPaths).SignAndNotarizeBinaries()
	if err != nil {
		return trace.Wrap(err, "failed to sign and notarize binaries")
	}

	return nil
}
