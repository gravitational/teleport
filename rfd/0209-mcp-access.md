---
authors: STeve Huang (xin.huang@goteleport.com)
state: draft
---

# RFD 0209 - MCP Access

## Required Approvers

* Engineering: @r0mant
* Product: @klizhentas

## What

Support zero-trust access for MCP servers.

## Why

Introduced in late 2024, Anthropic’s [Model Context Protocol
(MCP)](https://modelcontextprotocol.io/introduction) is a widely adopted,
open-source standard that enables language models to seamlessly interact with
external tools and data, enhancing their contextual capabilities.

However, MCP servers today are mostly operated locally without secure transport
or authorization. OAuth support was recently added to the specification, but as
of the time of writing, it is still new and not widely adopted.

With Teleport's support for MCP servers, users can:
- Host MCP servers on remote machines
- Secure transport with TLS
- Control access to specific MCP servers and specific tools defined by Teleport roles
- Track activity through Teleport's audit log

## Details

To expedite the initial rollout, the first iteration of MCP server support will
be implemented through App access.

### UX

#### editor - configure a filesystem MCP server and define RBAC
To configure a [filesystem MCP
server](https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem)
via Teleport app service:
```yaml
app_service:
  enabled: true
  apps:
  - name: "dev-files"
    description: "Shared files for developers"
    labels:
      env: "dev"
    mcp:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/dev/files"]
```

To create a role for admins that have full access to MCP servers:
```yaml
kind: role
version: v7
metadata:
  name: admin
spec:
  allow:
    app_labels:
      "*": "*"
```

To create a role for devs that can only access `dev` MCP servers and only have
read-only access to the filesystem:
```yaml
kind: role
version: v7
metadata:
  name: admin
spec:
  allow:
    app_labels:
      "env": "dev"
    # The name of the MCP tools to allow access to.
    # The wildcard character '*' matches any sequence of characters.
    # If the value begins with '^' and ends with '$', it is treated as a regular expression.
    mcp_tools:
    - get_*
    - search_files
    - ^(read|list)_.*$
```

#### access - configure Claude Desktop to use the MCP server via Teleport

First, to retrieve a list of allowed MCP servers:
```bash
$ tsh mcp ls
Name      Description                 Command Args
--------- --------------------------- ------- ------------------------------------------------
dev-files Shared files for developers npx     [-y @modelcontextprotocol/server-filesystem ...]
```

To show configuration information:
```bash
$ tsh mcp config dev-files
Use the following command to connect the "dev-files" MCP server:
tsh mcp connect dev-files

For example, to install it on Claude Desktop, add the following entry to
"mcpServers" in "claude_desktop_config.json", then restart Claude Desktop:
{
  "mcpServers": {
    "teleport-dev-files": {
      "command": "tsh",
      "args": [ "mcp", "connect", "dev-files" ]
    }
  }
}

Note that when tsh session expires, you may need to restart Claude Desktop after
logging in a new tsh session.
```

After following the instruction, Claude Desktop now shows a list of filesystem
tools that are from server "teleport-dev-files".

![Claude Desktop tools](assets/0209-claude-desktop-tools.png)

Now start chatting with Claude to use these tools. For example, "can you
retrieve me the content of file xxxx?", "can you search files related to xxxx?".

If Claude is asked to perform a write operation, the tool invocation receives an
access-denied error, and Claude understands the operation is denied by Teleport
based on the context provided in the error.

#### auditor - track MCP server usage

There are a few audit events related to MCP server sessions:
- `app.session.start`/`app.session.end`: A start event and an end event is
  expected per session from client. A new field `app_type` is added and MCP
  sessions will hold the value of `mcp`.
- `mcp.session.notification`: Notifications that clients send to the server,
  like `notifications/initialized`.
- `mcp.session.request`: Requests that clients send to the server, like
  `initialize`, `tools/call`. Basic discovery calls like `tools/list`,
  `resources/list` are not recorded. `tools/call` that are access denied will also
  have related errors recorded in the event.
  

### Implementation details

TODO overview mermaid diagram

#### Implementation - transport

TODO

#### Implementation - mcp handler with JSON-RPC

#### Implementation - tsh

#### Compatability with App access

TODO

### Usage reporting

MCP sessions will use `mcp` as the session type for the prehog session start events.

### Future improvements

One big improvement we can make is to move MCP servers to its own service
instead of relying on App service, if we want to expand MCP usage. There are
many other improvements that are being left out in the initial implementation.

#### Improvement - environment variable support

Many MCP servers require extra environment variables to run. For example, the
[Slack](https://github.com/modelcontextprotocol/servers/tree/main/src/slack) MCP
server requires `SLACK_BOT_TOKEN` and a few other environment variables.

The initial implementation will not support specifying environment variables in
the app definition as it might become a security concern when secrets like
`SLACK_BOT_TOKEN` are saved in the app definition.

Users can still work around this limitation by setting these environment
variables when running the Teleport process, since launching the MCP server will
inherit the Teleport process's environment by default.

#### Improvement - filter "tools/list"

The initial implementation can deny access to specific tools through
"tools/call". However, users can still see all available tools from the MCP
server, including the ones they don't have access to.

Ideally, the response of "tools/list" should be filtered so only allowed tools
are present. Special handling on "notifications/tools/list_changed" may also be
necessary.

#### Improvement - AI tool config handling

`tsh mcp config` only shows a sample config for Claude Desktop. A better UX
would be providing flags/sub commands to automatically detect and update the
config directly, even prompting for restart of the Claude Desktop.

In addition, other AI tools like Cursor, Zed, etc. can be added for support.

#### Improvement - handle expired tsh session

MCP server errors are permanent in Claude Desktop. When `tsh` session expires,
the user has to run `tsh login` then restart Claude Desktop.

To properly handle this, a mini JSON-RPC server should be served locally that
ensures a good connection with Claude Desktop. When `tsh` session expires, the
local JSON-RPC should instruct the user to perform a `tsh login` while
re-establish the connection with Proxy when possible. Note that the local
JSON-RPC server does NOT need to understand/implement the full MCP protocol as
it forwards the protocol to backend when things are good and rejects the request
with hint when `tsh` session expires.

#### Improvement - non-stdio transports

Besides stdio as the transport of choice, MCP spec also outlines [streamable
HTTP
transport](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#streamable-http)
and possibility of custom transports. Support to non-stdio transports can be
added to Teleport when they are more widely used.

### Security

TODO