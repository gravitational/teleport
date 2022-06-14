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

package crd

import (
	"context"
	"embed"
	"io/fs"

	"github.com/go-logr/logr"
	"github.com/gravitational/trace"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

func Upsert(ctx context.Context, setupLog logr.Logger, crdFS embed.FS, client client.Client) error {
	// Decode embedded CRDs.
	return fs.WalkDir(crdFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		contents, err := crdFS.ReadFile(path)
		if err != nil {
			return trace.Wrap(err)
		}

		var crd apiextv1.CustomResourceDefinition
		if err := yaml.Unmarshal(contents, &crd); err != nil {
			return trace.Wrap(err)
		}

		setupLog.Info("installing crd", "path", path, "metaName", crd.GetObjectMeta().GetName())

		_, err = controllerutil.CreateOrPatch(ctx, client, &crd, func() error {
			// we always install the current version
			// TODO(marco): merge CRDs
			return nil
		})
		return trace.Wrap(err)
	})
}
