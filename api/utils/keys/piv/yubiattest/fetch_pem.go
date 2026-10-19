//go:build ignore

// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"io"
	"net/http"
	"os"
)

func main() {
	fetch("https://developers.yubico.com/PKI/yubico-ca-1.pem", "yubico-ca-1.pem")
	fetch("https://developers.yubico.com/PKI/yubico-fido-ca-1.pem", "yubico-fido-ca-1.pem")
	fetch("https://developers.yubico.com/PKI/yubico-piv-ca-1.pem", "yubico-piv-ca-1.pem")
	fetch("https://developers.yubico.com/PKI/yubico-intermediate.pem", "yubico-intermediate.pem")
}

func fetch(url string, file string) {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(http.MaxBytesReader(nil, resp.Body, 64*1024))
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(file, b, 0o644); err != nil {
		panic(err)
	}
}
