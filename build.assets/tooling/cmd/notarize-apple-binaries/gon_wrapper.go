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
