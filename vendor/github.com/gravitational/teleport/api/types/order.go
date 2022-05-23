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

package types

// EventOrder is an ordering of events, either ascending or descending.
type EventOrder int

// EventOrderAscending is an ascending event order.
// In essence, events go from oldest to newest.
const EventOrderAscending = 0

// EventOrderDescending is an descending event order.
// In this ordering events go from newest to oldest.
const EventOrderDescending = 1
