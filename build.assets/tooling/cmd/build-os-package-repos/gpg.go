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
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type GPG struct{}

// Instantiates GPG, ensuring all system requirements for using GPG are fulfilled
func NewGPG() (*GPG, error) {
	g := &GPG{}

	err := g.ensureFirstRunHasOccurred()
	if err != nil {
		return nil, trace.Wrap(err, "failed to setup GPG")
	}

	err = g.ensureSecretKeyExists()
	if err != nil {
		return nil, trace.Wrap(err, "failed to ensure a secret key exists")
	}

	return g, nil
}

// The first time GPG is run for a user with any "meaningful" arguments it will
// generate several files and log what it created to stdout. These logs can
// disrupt parsing of GPG command outputs, so we force it to happen here, once,
// rather than try and handle it on each GPG call.
func (*GPG) ensureFirstRunHasOccurred() error {
	_, err := BuildAndRunCommand("gpg", "--fingerprint")
	if err != nil {
		return trace.Wrap(err, "failed to ensure GPG has been ran once")
	}

	return nil
}

func (*GPG) ensureSecretKeyExists() error {
	output, err := BuildAndRunCommand("gpg", "--list-secret-keys", "--with-colons")
	if err != nil {
		return trace.Wrap(err, "failed to ensure GPG secret key exists")
	}

	outputLineCount := strings.Count(output, "\n")
	if outputLineCount < 1 {
		return trace.Errorf("failed to find a GPG secret key")
	}

	return nil
}

// Creates a detached, armored signature for the provided file using the default GPG key
func (*GPG) SignFile(filePath string) error {
	// While this could be done via a Go module, the x/crypto/openpgp library has been frozen
	// and deprecated for almost 18 months. Others exist, but given the security implications of
	// using a less reputable Go module I've decided to just call `gpg` via shell instead.
	// Additionally this works and is just _so easy_ that it's probably not worth the effort to
	// use another library that reinvents the wheel.
	logrus.Debugf("Signing repo metadata at %q", filePath)

	// gpg --batch --yes --detach-sign --armor <filePath>
	_, err := BuildAndRunCommand("gpg", "--batch", "--yes", "--detach-sign", "--armor", filePath)
	if err != nil {
		return trace.Wrap(err, "failed to run GPG signing command on %q", filePath)
	}

	return nil
}

// Get the armored default public GPG key, ready to be written to a file
func (*GPG) GetPublicKey() (string, error) {
	// For reference here is how another company formats their key:
	// https://download.docker.com/linux/rhel/gpg
	logrus.Debug("Attempting to get the default public GPG key")

	key, err := BuildAndRunCommand("gpg", "--export", "--armor", "--no-version")
	if err != nil {
		return "", trace.Wrap(err, "failed to export the default public GPG key")
	}

	return key, nil
}

func (g *GPG) WritePublicKeyToFile(filePath string) error {
	logrus.Debugf("Writing the default armored public GPG key to %q", filePath)

	key, err := g.GetPublicKey()
	if err != nil {
		return trace.Wrap(err, "failed to retrieve public key")
	}

	err = os.WriteFile(filePath, []byte(key), 0664)
	if err != nil {
		return trace.Wrap(err, "failed to write key to %q", filePath)
	}

	return nil
}
