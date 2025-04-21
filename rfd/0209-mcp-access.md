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
- Control access to specific MCP servers and specific tools
- Track activity through Teleport's audit log

## Details

### UX


