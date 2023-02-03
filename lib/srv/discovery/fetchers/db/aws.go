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

package db

import (
	"github.com/gravitational/teleport/api/types"
)

// awsFetcher is the common base for AWS database fetchers.
type awsFetcher struct {
}

// Cloud returns the cloud the fetcher is operating.
func (f *awsFetcher) Cloud() string {
	return types.CloudAWS
}

// ResourceType identifies the resource type the fetcher is returning.
func (f *awsFetcher) ResourceType() string {
	return types.KindDatabase
}

// maxAWSPages is the maximum number of pages to iterate over when fetching aws
// databases.
const maxAWSPages = 10
