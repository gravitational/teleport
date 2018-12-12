/*
Copyright 2018-2019 Gravitational, Inc.

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

// Package cache implements event-driven cache layer
// that is used by auth servers, proxies and nodes.
//
// The cache fetches resources and then subscribes
// to the events watcher to receive updates.
//
// This approach allows cache to be up to date without
// time based expiration and avoid re-fetching all
// resources reducing bandwitdh.
//
// There are two types of cache backends used:
//
// * SQLite-based in-memory used for auth nodes
// * SQLite-based on disk persistent cache for nodes and proxies
// providing resilliency in the face of auth servers failures.
package cache
