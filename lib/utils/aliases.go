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

package utils

import (
	"github.com/gravitational/teleport/api/utils"
)

// The following util functions have been moved to /api/utils, and are now
// imported here for backwards compatibility.

// slices.go
var (
	CopyByteSlice     = utils.CopyByteSlice
	CopyByteSlices    = utils.CopyByteSlices
	StringSlicesEqual = utils.StringSlicesEqual
	SliceContainsStr  = utils.SliceContainsStr
	Deduplicate       = utils.Deduplicate
)

// strings.go
type Strings = utils.Strings

var CopyStrings = utils.CopyStrings

// regexp.go
var (
	ReplaceRegexp = utils.ReplaceRegexp
	GlobToRegexp  = utils.GlobToRegexp
)

// time.go
var (
	UTC                   = utils.UTC
	HumanTimeFormatString = utils.HumanTimeFormatString
	HumanTimeFormat       = utils.HumanTimeFormat
)

// utils.go
var (
	ParseBool = utils.ParseBool
)
