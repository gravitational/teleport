//go:build eimports

// Copyright 2022 Gravitational, Inc
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

package teleport

// Hold a few imports done exclusively by e/, so tidying that doesn't have
// access to it (like Dependabot) doesn't wrongly remove them.
// Any import that is present only in this file, but not in e/, can be safely
// removed.
import (
	_ "github.com/beevik/etree"          // hold for e/
	_ "github.com/go-piv/piv-go/piv"     // hold for e/
	_ "github.com/gravitational/form"    // hold for e/
	_ "github.com/gravitational/license" // hold for e/
	_ "gopkg.in/check.v1"                // hold for e/
)
