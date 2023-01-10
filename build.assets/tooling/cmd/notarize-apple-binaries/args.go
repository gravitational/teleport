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
	"debug/macho"
	"flag"
	"fmt"
	"os"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type Args struct {
	*LoggerConfig
	AppleUsername string
	ApplePassword string
	BinaryPaths   []string
}

func NewArgs() *Args {
	args := &Args{}
	args.LoggerConfig = NewLoggerConfig()
	flag.StringVar(&args.AppleUsername, "apple-username", "", "Apple Connect username used for notarization")
	flag.StringVar(&args.ApplePassword, "apple-password", "", "Apple Connect password used for notarization")

	return args
}

func (a *Args) Check() error {
	err := a.LoggerConfig.Check()
	if err != nil {
		return trace.Wrap(err, "failed to validate the logger config")
	}

	err = a.validateAppleUsername()
	if err != nil {
		return trace.Wrap(err, "failed to validate the apple-username flag")
	}

	err = a.validateApplePassword()
	if err != nil {
		return trace.Wrap(err, "failed to validate the apple-password flag")
	}

	err = a.validateBinaryPaths()
	if err != nil {
		return trace.Wrap(err, "failed to validate binary path(s)")
	}

	// It might be worth adding an actual login check here in the future

	return nil
}

func (a *Args) validateAppleUsername() error {
	if a.AppleUsername == "" {
		return trace.BadParameter("the apple-username flag should not be empty")
	}

	return nil
}

func (a *Args) validateApplePassword() error {
	if a.ApplePassword == "" {
		return trace.BadParameter("the apple-password flag should not be empty")
	}

	return nil
}

func (a *Args) validateBinaryPaths() error {
	// Check for minimum arg count and load the parameters into the struct
	if len(a.BinaryPaths) == 0 {
		return trace.BadParameter("the path to at least one binary is required")
	}

	// Validate each path
	binaryPaths := flag.Args()
	for _, binaryPath := range binaryPaths {
		logrus.Debugf("Validating binary path %q...", binaryPath)
		err := a.verifyFileIsValidForSigning(binaryPath)
		if err != nil {
			return trace.Wrap(err, "file %q failed binary validation", binaryPath)
		}
	}

	return nil
}

// Returns an error if the file is not valid for signing
func (a *Args) verifyFileIsValidForSigning(filePath string) error {
	err := a.verifyPathIsNotDirectory(filePath)
	if err != nil {
		return trace.Wrap(err, "filesystem path %q is a directory")
	}

	err = a.verifyFileIsAppleBinary(filePath)
	if err != nil {
		return trace.Wrap(err, "file %q is not an Apple binary", filePath)
	}

	return nil
}

// Returns an error if the provided binary path is actually a directory
func (a *Args) verifyPathIsNotDirectory(filePath string) error {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return trace.Wrap(err, "failed to stat file system information for path %q", filePath)
	}

	if fileInfo.IsDir() {
		return trace.Errorf("filesystem path %q is a directory and does not point to a binary", filePath)
	}

	return nil
}

// Returns an error if the file is not a valid Apple binary
// Effectively does `file $BINARY | grep -ic 'mach-o'`
func (a *Args) verifyFileIsAppleBinary(filePath string) error {
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

func usage() {
	fmt.Printf("Usage: %s [OPTIONS] BINARIES...\n", flag.CommandLine.Name())
	fmt.Println()
	flag.PrintDefaults()
}

func parseArgs() (*Args, error) {
	flag.Usage = usage

	args := NewArgs()
	flag.Parse()
	args.BinaryPaths = flag.Args()

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
