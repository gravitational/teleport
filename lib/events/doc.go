/*
Copyright 2017 Gravitational, Inc.

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
Package events currently implements the audit log using a simple filesystem backend.
"Implements" means it implements events.IAuditLog interface (see events/api.go)

The main log files are saved as:
	/var/lib/teleport/log/<date>.log

Each session has its own session log stored as two files
	/var/lib/teleport/log/<session-id>.session.log
	/var/lib/teleport/log/<session-id>.session.bytes

Where:
	- .session.log   (same events as in the main log, but related to the session)
	- .session.bytes (recorded session bytes: PTY IO)

The log file is rotated every 24 hours. The old files must be cleaned
up or archived by an external tool.

Log file format:
utc_date,action,json_fields

Common JSON fields
- user       : teleport user
- login      : server OS login, the user logged in as
- addr.local : server address:port
- addr.remote: connected client's address:port
- sid        : session ID (GUID format)

Examples:
2016-04-25 22:37:29 +0000 UTC,session.start,{"addr.local":"127.0.0.1:3022","addr.remote":"127.0.0.1:35732","login":"root","sid":"4a9d97de-0b36-11e6-a0b3-d8cb8ae5080e","user":"vincent"}
2016-04-25 22:54:31 +0000 UTC,exec,{"addr.local":"127.0.0.1:3022","addr.remote":"127.0.0.1:35949","command":"-bash -c ls /","login":"root","user":"vincent"}
*/
package events
