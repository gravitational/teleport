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
)

const (
	DevelopereIdentity string = "0FFD3E3413AB4C599C53FBB1D8CA690915E33D83"
	BundleId           string = "com.gravitational.teleport"
)

type GonWrapper struct {
	ctx    context.Context
	logger hclog.Logger
	config *GonConfig
}

func NewGonWrapper(config *GonConfig) *GonWrapper {
	return &GonWrapper{
		ctx:    context.Background(),
		logger: NewHCLogLogrusAdapter(),
		config: config,
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

	logrus.Info("Signing and notarization complete!")
	return nil
}

func (gw *GonWrapper) SignBinaries() error {
	logrus.Infof("Signing binaries %v...", gw.config.BinaryPaths)
	err := sign.Sign(gw.ctx, &sign.Options{
		Files:    gw.config.BinaryPaths,
		Identity: DevelopereIdentity,
		Logger:   gw.logger,
	})

	if err != nil {
		return trace.Wrap(err, "gon failed to sign binaries %v", gw.config.BinaryPaths)
	}

	return nil
}

func (gw *GonWrapper) ZipBinaries() (string, error) {
	zipFileName := "notarization.zip"
	logrus.Infof("Zipping binaries into %q for notarization upload...", zipFileName)
	tmpDir, err := os.MkdirTemp("", "gon-zip-directory-*")
	if err != nil {
		return "", trace.Wrap(err, "failed to create temporary directory for binary zipping")
	}

	outputPath := path.Join(tmpDir, zipFileName)
	logrus.Debugf("Using binary zip path %q", outputPath)

	err = zip.Zip(gw.ctx, &zip.Options{
		Files:      gw.config.BinaryPaths,
		OutputPath: outputPath,
		Logger:     gw.logger,
	})

	if err != nil {
		os.RemoveAll(tmpDir)
		return "", trace.Wrap(err, "gon failed to zip binaries %v to zip %q", gw.config.BinaryPaths, outputPath)
	}

	return outputPath, nil
}

func (gw *GonWrapper) NotarizeBinaries(zipPath string) error {
	logrus.Infof("Uploading %q to Apple for notarization ticket issuance. This may take awhile...", zipPath)
	notarizationInfo, err := notarize.Notarize(gw.ctx, &notarize.Options{
		File:     zipPath,
		BundleId: BundleId,
		Username: gw.config.AppleUsername,
		Password: gw.config.ApplePassword,
		Logger:   gw.logger,
	})
	if err != nil {
		return trace.Wrap(err, "gon failed to notarize binaries %v in zip %q with notarization info %+v",
			gw.config.BinaryPaths, zipPath, notarizationInfo)
	}

	return nil
}
