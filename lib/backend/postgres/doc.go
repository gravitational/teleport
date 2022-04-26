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
Package postgres implements a SQL backend for PostgreSQL and CockroachDB.

Schema

The database schema consists of three tables: item, lease, and event.
    ┌──────────┐ ┌──────────┐ ┌──────────┐
    │  item    │ │  lease   │ │  event   │
    ├──────────┤ ├──────────┤ ├──────────┤
    │* key     │ │* key     │ │* eventid │
    │* id      │ │  id      │ │  created │
    │  value   │ │  expires │ │  key     │
    │          │ │          │ │  id      │
    │          │ │          │ │  type    │
    └──────────┘ └──────────┘ └──────────┘

The item table contains the backend item's value and is insert-only. The table
supports multiple items per key. Updates to an item's value creates a new
record with an ID greater than the most recent record.

The lease table contains the backend item's active record, which may have already
expired. Active leases have a null expires value or expires is greater than the
current time.

The event table contains events for all changes to backend items and is keyed by an
autoincrementing integer (may not be a sequence/will contain gaps). The event's
type represents the value of types.OpType.

The design allows for items to be updated before an event for previous item has
been emitted without duplicating storage for value.

*/
package postgres
