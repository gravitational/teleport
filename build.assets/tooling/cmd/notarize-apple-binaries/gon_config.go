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
	"os"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type GonConfig struct {
	AppleUsername string
	ApplePassword string
	BinaryPaths   []string
}

func NewGonConfig() *GonConfig {
	gc := &GonConfig{}
	flag.StringVar(&gc.AppleUsername, "apple-username", "", "Apple Connect username used for notarization")
	flag.StringVar(&gc.ApplePassword, "apple-password", "", "Apple Connect password used for notarization")

	return gc
}

func (gc *GonConfig) Check() error {
	err := gc.validateAppleUsername()
	if err != nil {
		return trace.Wrap(err, "failed to validate the apple-username flag")
	}

	err = gc.validateApplePassword()
	if err != nil {
		return trace.Wrap(err, "failed to validate the apple-password flag")
	}

	err = gc.validateBinaryPaths()
	if err != nil {
		return trace.Wrap(err, "failed to validate binary path(s)")
	}

	return nil

	// It might be worth adding an actual login check here in the future
}

func (gc *GonConfig) validateAppleUsername() error {
	if gc.AppleUsername == "" {
		return trace.BadParameter("the apple-username flag should not be empty")
	}

	return nil
}

func (gc *GonConfig) validateApplePassword() error {
	if gc.ApplePassword == "" {
		return trace.BadParameter("the apple-password flag should not be empty")
	}

	return nil
}

func (gc *GonConfig) validateBinaryPaths() error {
	// Check for minimum arg count and load the parameters into the struct
	if flag.NArg() == 0 {
		return trace.BadParameter("the path to at least one binary is required")
	}
	gc.BinaryPaths = flag.Args()

	// Validate each path
	binaryPaths := flag.Args()
	for _, binaryPath := range binaryPaths {
		logrus.Debugf("Validating binary path %q...", binaryPath)
		err := gc.verifyFileIsValidForSigning(binaryPath)
		if err != nil {
			return trace.Wrap(err, "file %q failed binary validation", binaryPath)
		}
	}

	return nil
}

// Returns an error if the file is not valid for signing
func (gc *GonConfig) verifyFileIsValidForSigning(filePath string) error {
	err := gc.verifyPathIsNotDirectory(filePath)
	if err != nil {
		return trace.Wrap(err, "filesystem path %q is a directory")
	}

	err = gc.verifyFileIsAppleBinary(filePath)
	if err != nil {
		return trace.Wrap(err, "file %q is not an Apple binary", filePath)
	}

	return nil
}

// Returns an error if the provided binary path is actually a directory
func (gc *GonConfig) verifyPathIsNotDirectory(filePath string) error {
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
func (gc *GonConfig) verifyFileIsAppleBinary(filePath string) error {
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
