// Copyright 2018 Google LLC
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
//
////////////////////////////////////////////////////////////////////////////////

// Package internal provides a coordination point for package keyset, package
// insecurecleartextkeyset, and package testkeyset.  internal must only be
// imported by these three packages.
package internal

// KeysetHandle is a raw constructor of keyset.Handle.
var KeysetHandle interface{}

// KeysetMaterial returns the key material contained in a keyset.Handle.
var KeysetMaterial interface{}
