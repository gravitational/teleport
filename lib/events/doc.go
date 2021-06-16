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
Package events implements the audit log interface events.IAuditLog
using filesystem backend.

Audit logs
----------

Audit logs are events associated with user logins, server access
and session log events like session.start.

Example audit log event:

{"addr.local":"172.10.1.20:3022",
 "addr.remote":"172.10.1.254:58866",
 "event":"session.start",
 "login":"root",
 "user":"klizhentas@gmail.com"
}

Session Logs
------------

Session logs are a series of events and recorded SSH interactive session playback.

Example session log event:

{
  "time":"2018-01-04T02:12:40.245Z",
  "event":"print",
  "bytes":936,
  "ms":40962,
  "offset":16842,
  "ei":31,
  "ci":29
}

Print event fields
------------------

Print event specifies session output - PTY io recorded by Teleport node or Proxy
based on the configuration.

* "offset" is an offset in bytes from a start of a session
* "ms" is a delay in milliseconds from the last event occurred
* "ci" is a chunk index ordering only print events
* "ei" is an event index ordering events from the first one

As in example of print event above, "ei" - is a session event index - 31,
while "ci" is a chunk index - meaning that this event is 29th in a row of print events.

Client streaming session logs
------------------------------

Session related logs are delivered in order defined by clients.
Every event is ordered and has a session-local index, every next event has index incremented.

Client delivers session events in batches, where every event in the batch
is guaranteed to be in continuous order (e.g. no cases with events
delivered in a single batch to have missing event or chunk index).

Disk File format
----------------

On disk file format is designed to be compatible with NFS filesystems and provides
guarantee that only one auth server writes to the file at a time.

Main Audit Log Format
=====================

The main log files are saved as:

	/var/lib/teleport/log/<auth-server-id>/<date>.log

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

Session log file format
=======================

Each session has its own session log stored as several files:

Index file contains a list of event files and chunks files associated with a session:

	/var/lib/teleport/log/sessions/<auth-server-id>/<session-id>.index

The format of the index file contains of two or more lines with pointers to other files:

{"file_name":"<session-id>-<first-event-in-file-index>.events","type":"events","index":<first-event-in-file-index>}
{"file_name":"<session-id>-<first-chunk-in-file-offset>.chunks","type":"chunks","offset":<first-chunk-in-file-offset>}

Files:

	/var/lib/teleport/log/<auth-server-id>/<session-id>-<first-event-in-file-index>.events
	/var/lib/teleport/log/<auth-server-id>/<session-id>-<first-chunk-in-file-offset>.chunks

Where:
	- .events   (same events as in the main log, but related to the session)
	- .chunks (recorded session bytes: PTY IO)

Examples
~~~~~~~~

**Single auth server**

In the simplest case, single auth server a1 log for a single session id s1
will consist of three files:

/var/lib/teleport/a1/s1.index

With contents:

{"file_name":"s1-0.events","type":"events","index":0}
{"file_name":"s1-0.chunks","type":"chunks","offset":0}

This means that all session events are located in s1-0.events file starting from
the first event with index 0 and all chunks are located in file s1-0.chunks file
with the byte offset from the start - 0.

File with session events /var/lib/teleport/a1/s1-0.events will contain:

{"ei":0,"event":"session.start", ...}
{"ei":1,"event":"resize",...}
{"ei":2,"ci":0, "event":"print","bytes":40,"offset":0}
{"ei":3,"event":"session.end", ...}

File with recorded session /var/lib/teleport/a1/s1-0.chunks will contain 40 bytes
emitted by print event with chunk index 0

**Multiple Auth Servers**

In High Availability mode scenario, multiple auth servers will be
 deployed behind a load balancer.

Any auth server can go down during session and clients will retry the delivery
to the other auth server.

Both auth servers have mounted /var/lib/teleport/log as a shared NFS folder.

To make sure that only one auth server writes to a file at a time,
each auth server writes to it's own file in a sub folder named
with host UUID of the server.

Client sends the chunks of events related to the session s1 in order,
but load balancer sends first batch of event to the first server a1,
and the second batch of event to the second server a2.

Server a1 will produce the following file:

/var/lib/teleport/a1/s1.index

With contents:

{"file_name":"s1-0.events","type":"events","index":0}
{"file_name":"s1-0.chunks","type":"chunks","offset":0}

Events file /var/lib/teleport/a1/s1-0.events will contain:

{"ei":0,"event":"session.start", ...}
{"ei":1,"event":"resize",...}
{"ei":2,"ci":0, "event":"print","bytes":40,"offset":0}

Events file /var/lib/teleport/a1/s1-0.chunks will contain 40 bytes
emitted by print event with chunk index.

Server a2 will produce the following file:

/var/lib/teleport/a2/s1.index

With contents:

{"file_name":"s1-3.events","type":"events","index":3}
{"file_name":"s1-40.chunks","type":"chunks","offset":40}

Events file /var/lib/teleport/a2/s1-4.events will contain:

{"ei":3,"ci":1, "event":"print","bytes":15,"ms":713,"offset":40}
{"ei":4,"event":"session.end", ...}

Events file /var/lib/teleport/a2/s1-40.chunks will contain 15 bytes emitted
by print event with chunk index 1 and comes after delay of 713 milliseconds.

Offset 40 indicates that the first chunk stored in the file s1-40.chunks
comes at an offset of 40 bytes from the start of the session.

Log Search and Playback
-----------------------

Log search and playback is aware of multiple auth servers, merges
indexes, event streams stored on multiple auth servers.

*/
package events
