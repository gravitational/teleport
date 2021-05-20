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

// Package protocol implements parts of MySQL wire protocol which are needed
// for the service to be able to interpret the protocol messages but are not
// readily available in the convenient form in the vendored MySQL library.
//
// For example, reading protocol packets from connections, parsing them and
// writing them to connections.
//
// The following resources are helpful to understand protocol details.
//
// Packet structure:
//   https://dev.mysql.com/doc/internals/en/mysql-packet.html
//
// Generic response packets:
//   https://dev.mysql.com/doc/internals/en/generic-response-packets.html
//
// Packets sent in the command phase:
//   https://dev.mysql.com/doc/internals/en/command-phase.html
package protocol
