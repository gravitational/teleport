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

/*
Package sqlbk implements a storage backend SQL databases.

The backend requires a Driver, which is an abstraction for communicating with a
specific database platforms such as PostgreSQL. A Driver opens a connection pool
that communicates with a database instance through a DB interface. A DB exposes
an interface to create transactions with cancellation through a Tx interface.

	Driver -> DB -> Tx

Testing

Test a Driver implementation using the TestDriver package function. The test
will configure the driver for use with a test backend and execute the backend
test suite. See driver implementations for details about configuring tests.


*/
package sqlbk
