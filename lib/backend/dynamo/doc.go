// Copyright 2021 Gravitational, Inc
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

/*
Package dynamo implements DynamoDB storage backend
for Teleport auth service, similar to etcd backend.

dynamo package implements the DynamoDB storage back-end for the
auth server. Originally contributed by https://github.com/apestel

limitations:

* Paging is not implemented, hence all range operations are limited
  to 1MB result set
*/
package dynamo
