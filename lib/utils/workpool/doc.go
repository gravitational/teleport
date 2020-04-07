/*
Copyright 2020 Gravitational, Inc.

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

// Package workpool provies the `Pool` type which functions as a means
// of managing the number of concurrent workers,
// grouped by key.  You can think of this type as functioning
// like a collection of semaphores, except that multiple distinct resources may
// exist (distinguished by keys), and the target concurrent worker count may
// change at runtime.  The basic usage pattern is as follows:
//
// 1. The desired number of workers for a given key is specified
// or updated via Pool.Set.
//
// 2. Workers are spawned as leases become available on Pool.Acquire.
//
// 3. Workers relenquish their leases when they finish their work
// by calling Lease.Release.
//
// 4. New leases become available as old leases are relenquished, or
// as the target concurrent lease count increases.
//
// This is a generalization of logic originally written to manage the number
// of concurrent reversetunnel agents per proxy endpoint.
package workpool
