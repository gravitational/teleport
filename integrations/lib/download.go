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

package lib

import (
	"context"
	"io"
	"net/http"

	"github.com/gravitational/trace"
)

// DownloadAndCheck gets a file from the Internet and checks its SHA256 sum.
func DownloadAndCheck(ctx context.Context, url string, out io.Writer, checksum SHA256Sum) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	sha256 := NewSHA256()
	if _, err = io.Copy(out, io.TeeReader(resp.Body, sha256)); err != nil {
		return trace.Wrap(err)
	}

	if sha256.Sum() != checksum {
		return trace.CompareFailed("sha256 sum of downloaded file does not match")
	}
	return nil
}
