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
	"context"
	"os"
	"path"

	"github.com/gravitational/trace"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/gon/notarize"
	"github.com/mitchellh/gon/package/zip"
	"github.com/mitchellh/gon/sign"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/build.assets/tooling/lib/logging"
)

type GonWrapper struct {
	ctx           context.Context
	logger        hclog.Logger
	AppleUsername string
	ApplePassword string
	DeveloperID   string
	BundleID      string
	BinaryPaths   []string
}

func NewGonWrapper(appleUsername, applePassword, developerID, bundleID string, BinaryPaths []string) *GonWrapper {
	return &GonWrapper{
		ctx:           context.Background(),
		logger:        logging.NewHCLogLogrusAdapter(logrus.StandardLogger()),
		AppleUsername: appleUsername,
		ApplePassword: applePassword,
		DeveloperID:   developerID,
		BundleID:      bundleID,
		BinaryPaths:   BinaryPaths,
	}
}

func (gw *GonWrapper) SignAndNotarizeBinaries() error {
	err := gw.SignBinaries()
	if err != nil {
		return trace.Wrap(err, "failed to sign binaries")
	}

	zipPath, err := gw.ZipBinaries()
	if err != nil {
		return trace.Wrap(err, "failed to zip binaries for notarization")
	}
	defer os.RemoveAll(path.Dir(zipPath))

	err = gw.NotarizeBinaries(zipPath)
	if err != nil {
		return trace.Wrap(err, "failed to notarize binaries")
	}

	gw.logger.Info("Signing and notarization complete!")
	return nil
}

func (gw *GonWrapper) SignBinaries() error {
	gw.logger.Info("Signing binaries %v...", gw.BinaryPaths)
	err := sign.Sign(gw.ctx, &sign.Options{
		Files:    gw.BinaryPaths,
		Identity: gw.DeveloperID,
		Logger:   gw.logger,
	})

	if err != nil {
		return trace.Wrap(err, "gon failed to sign binaries %v", gw.BinaryPaths)
	}

	return nil
}

func (gw *GonWrapper) ZipBinaries() (string, error) {
	zipFileName := "notarization.zip"
	gw.logger.Info("Zipping binaries into %q for notarization upload...", zipFileName)
	tmpDir, err := os.MkdirTemp("", "gon-zip-directory-*")
	if err != nil {
		return "", trace.Wrap(err, "failed to create temporary directory for binary zipping")
	}

	outputPath := path.Join(tmpDir, zipFileName)
	gw.logger.Debug("Using binary zip path %q", outputPath)

	err = zip.Zip(gw.ctx, &zip.Options{
		Files:      gw.BinaryPaths,
		OutputPath: outputPath,
		Logger:     gw.logger,
	})

	if err != nil {
		os.RemoveAll(tmpDir)
		return "", trace.Wrap(err, "gon failed to zip binaries %v to zip %q", gw.BinaryPaths, outputPath)
	}

	return outputPath, nil
}

func (gw *GonWrapper) NotarizeBinaries(zipPath string) error {
	gw.logger.Info("Uploading %q to Apple for notarization ticket issuance. This may take awhile...", zipPath)
	notarizationInfo, err := notarize.Notarize(gw.ctx, &notarize.Options{
		File:     zipPath,
		BundleId: gw.BundleID,
		Username: gw.AppleUsername,
		Password: gw.ApplePassword,
		Logger:   gw.logger,
	})
	if err != nil {
		return trace.Wrap(err, "gon failed to notarize binaries %v in zip %q with notarization info %+v",
			gw.BinaryPaths, zipPath, notarizationInfo)
	}

	return nil
}
