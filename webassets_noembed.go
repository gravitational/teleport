//go:build !webassets_embed

/*
Copyright 2021 Gravitational, Inc.

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

package teleport

import (
	"net/http"

	"github.com/gravitational/trace"
)

const webAssetsMissingError = "the teleport binary was built without web assets, try building with `make release`"

// NewWebAssetsFilesystem is a no-op in this build mode.
func NewWebAssetsFilesystem() (http.FileSystem, error) { //nolint:staticcheck // suppress 'never returns nil' as this is value is platform dependent
	return nil, trace.NotFound(webAssetsMissingError)
}
