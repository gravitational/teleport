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

package app

// TODO(russjones): Add test coverage.

// TODO(russjones): Add test that makes sure cookies are removed.
// TODO(russjones): Check that "newSession" rejects invalid schemes, like "postgres://".
// TODO(russjones): Make sure that an error handler is attached to
//   fwd.ServeHTTP and that upon error, it removed the session object.
// TODO(russjones): Add test to verify TTL is respected in sessionCache.get.
