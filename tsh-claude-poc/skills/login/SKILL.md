---
name: login
description: Login to Teleport clusters using tsh
user-invocable: true
allowed-tools: mcp__plugin_tsh_tsh__login, mcp__plugin_tsh_tsh__status
---

## When to use this skill

This skill is activated when you need to login to a Teleport cluster using tsh.

## Instructions

When the user asks to login to Teleport or when you detect they need authentication:

1. **ALWAYS use the `login` tool from the tsh MCP server**
2. Present the available cluster options to the user if they haven't specified which one

## Available Clusters

There are two Teleport clusters available:

1. **Local Development Cluster**
   - Proxy: `https://steve.teleport.test:34443`
   - User: `admin`
   - Auth method: Local user authentication

2. **Platform Cluster**
   - Proxy: `https://platform.teleport.sh:443`
   - Auth method: SSO (Single Sign-On)

## How to use the login tool

The `login` tool from the tsh MCP server handles Teleport cluster authentication.

### Parameters:

- **proxy_addr** (required): The Teleport proxy address
  - Examples: `"steve.teleport.test:34443"`, `"platform.teleport.sh:443"`

- **user** (optional): The username for local authentication
  - Required for local auth (e.g., `"admin"` for the local dev cluster)
  - Not needed for SSO authentication

- **sso** (required): Boolean indicating whether to use SSO authentication
  - `true` for SSO (platform.teleport.sh)
  - `false` for local user authentication (steve.teleport.test)

### Examples:

#### Login to local development cluster:
```
Tool: login (from tsh MCP server)
Parameters:
{
  "proxy_addr": "steve.teleport.test:34443",
  "user": "admin",
  "sso": false
}
```

#### Login to platform cluster with SSO:
```
Tool: login (from tsh MCP server)
Parameters:
{
  "proxy_addr": "platform.teleport.sh:443",
  "sso": true
}
```

## Workflow

1. If the user asks to login without specifying a cluster, ask which cluster they want to use:
   - "steve.teleport.test (local dev with admin user)"
   - "platform.teleport.sh (with SSO)"

2. Call the appropriate login tool with the correct parameters

3. After calling the login tool, inform the user that:
   - The login process has been initiated
   - They should complete authentication in their browser (for SSO) or Teleport Connect
   - They can verify their login status using the `status` tool

4. Optionally, after a moment, offer to check their login status using the `status` tool from the tsh MCP server

## Benefits

- **Simplified authentication**: One command to login to any cluster
- **SSO support**: Seamless integration with identity providers
- **Local development**: Quick access to local test clusters
- **Security**: Authentication handled through official tsh client

## Important notes

- The login tool initiates the authentication flow but doesn't complete it immediately
- Users must complete the authentication in their browser or Teleport Connect
- After login, use the `status` tool to verify the authentication was successful
- Login sessions have expiration times - check `valid_until` in the status output
